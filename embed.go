package main

import "embed"

// distFS holds the built frontend. It has to live next to web/ because
// go:embed cannot reach outside the directory of the file that declares it,
// which is also why the server takes an fs.FS instead of embedding assets
// itself.
//
//go:embed web/dist
var distFS embed.FS
