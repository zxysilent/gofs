package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/zxysilent/logs"
)

func main() {
	res, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if res.help {
		printHelp()
		return
	}
	if res.version {
		fmt.Printf("gofs %s\n", version)
		return
	}
	if res.completions != "" {
		printCompletions(res.completions)
		return
	}

	args := res.args
	initLogger(args.logFile)

	serve(args)
}

func initLogger(logFile string) {
	logs.SetLevel(logs.LevelInfo)
	if logFile != "" {
		logs.SetFile(logFile)
		logs.SetConsole(false)
	}
}

func serve(args Args) {
	var running atomic.Bool
	running.Store(true)

	server, err := newServer(args, &running)
	if err != nil {
		logs.Errorf("%v", err)
		os.Exit(1)
	}

	var servers []*http.Server
	for _, addr := range args.addrs {
		l, err := listen(addr, args.port)
		if err != nil {
			logs.Errorf("failed to bind `%s`: %v", addr.addr, err)
			os.Exit(1)
		}
		srv := &http.Server{Handler: server}
		servers = append(servers, srv)
		go func(l net.Listener, srv *http.Server) {
			if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
				logs.Errorf("%v", err)
			}
		}(l, srv)
	}

	logs.Info(printListening(args))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	running.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range servers {
		srv.Shutdown(ctx)
	}
}

func listen(addr BindAddr, port int) (net.Listener, error) {
	if addr.isIP {
		ip := net.ParseIP(addr.addr)
		network := "tcp6"
		if ip.To4() != nil {
			network = "tcp4"
		}
		return net.Listen(network, net.JoinHostPort(addr.addr, strconv.Itoa(port)))
	}
	// unix socket
	path := addr.addr
	if strings.HasPrefix(path, "@") {
		// abstract socket
		return net.Listen("unix", "\x00"+path[1:])
	}
	os.Remove(path)
	return net.Listen("unix", path)
}

func printListening(args Args) string {
	var urls []string
	for _, addr := range args.addrs {
		if addr.isIP {
			host := addr.addr
			if strings.Contains(host, ":") {
				host = "[" + host + "]"
			}
			protocol := "http"
			if args.tlsCert != "" {
				protocol = "https"
			}
			urls = append(urls, fmt.Sprintf("%s://%s:%d%s", protocol, host, args.port, args.uriPrefix))
		} else {
			urls = append(urls, addr.addr)
		}
	}
	if len(urls) == 1 {
		return "Listening on " + urls[0]
	}
	var sb strings.Builder
	sb.WriteString("Listening on:\n")
	for _, u := range urls {
		sb.WriteString("  ")
		sb.WriteString(u)
		sb.WriteString("\n")
	}
	return sb.String()
}

func printHelp() {
	fmt.Print(`gofs is a distinctive utility file server.

Usage: gofs [OPTIONS] [serve-path]

Arguments:
  [serve-path]  Specific path to serve [default: .]

Options:
  -c, --config <file>        Specify configuration file
  -b, --bind <addrs>         Specify bind address or unix socket
  -p, --port <port>          Specify port to listen on [default: 5000]
      --path-prefix <path>   Specify a path prefix
      --hidden <value>       Hide paths from directory listings, e.g. tmp,*.log,*.lock
  -a, --auth <rules>         Add auth roles, e.g. user:pass@/dir1:rw,/dir2
  -A, --allow-all            Allow all operations
      --allow-upload         Allow upload files/folders
      --allow-delete         Allow delete files/folders
      --allow-search         Allow search files/folders
      --allow-symlink        Allow symlink to files/folders outside root directory
      --allow-archive        Allow download folders as archive file
      --allow-hash           Allow ?hash query to get file sha256 hash
      --enable-cors          Enable CORS, sets ` + "`Access-Control-Allow-Origin: *`" + `
      --render-index         Serve index.html when requesting a directory, returns 404 if not found index.html
      --render-try-index     Serve index.html when requesting a directory, returns directory listing if not found index.html
      --render-spa           Serve SPA(Single Page Application)
      --assets <path>        Set the path to the assets directory for overriding the built-in assets
      --log-format <format>  Customize http log format
      --log-file <file>      Specify the file to save logs to, other than stdout/stderr
      --compress <level>     Set zip compress level [default: low] [possible values: none, low, medium, high]
      --completions <shell>  Print shell completion script for <shell> [possible values: bash, elvish, fish, powershell, zsh]
      --tls-cert <path>      Path to an SSL/TLS certificate to serve with HTTPS
      --tls-key <path>       Path to the SSL/TLS certificate's private key
  -h, --help                 Print help
  -V, --version              Print version
`)
}

func printCompletions(shell string) {
	// A minimal completion script (bash). Full completions are beyond the
	// scope of this reimplementation; we emit a functional bash script.
	if shell == "bash" {
		fmt.Print(`_gofs() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local opts="--config --bind --port --path-prefix --hidden --auth --allow-all --allow-upload --allow-delete --allow-search --allow-symlink --allow-archive --allow-hash --enable-cors --render-index --render-try-index --render-spa --assets --log-format --log-file --compress --completions --tls-cert --tls-key --help --version"
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _gofs gofs
`)
		return
	}
	fmt.Printf("# completion for %s is not implemented\n", shell)
}
