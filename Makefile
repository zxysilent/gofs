# gofs — 打包入口
#
# web/dist 不再提交到仓库，它由 `npm run build` 生成并内嵌进二进制，
# 所以构建前一定要先构建前端（`make build` 会自动做），否则二进制启动时
# 会报 "read built frontend index.html"。

GO      ?= go
NPM     ?= npm
BIN     ?= gofs
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help web build build-go test vet fmt run clean

help:
	@echo "make web        构建前端 (web/dist)"
	@echo "make build      构建前端 + 二进制 (VERSION=$(VERSION))"
	@echo "make build-go   只编译二进制，要求 web/dist 已存在"
	@echo "make test       go test -race ./..."
	@echo "make vet        go vet ./..."
	@echo "make fmt        gofmt -w"
	@echo "make run        本地运行，服务当前目录"
	@echo "make clean      清理二进制与 web/dist"

web:
	cd web && $(NPM) ci && $(NPM) run build

build: web build-go

build-go:
	CGO_ENABLED=0 $(GO) build -trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) .

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $$(git ls-files '*.go')

run: build-go
	./$(BIN) -A .

clean:
	rm -f $(BIN) $(BIN).exe
	# 只删产物，保留 .gitkeep：它是 //go:embed all:web/dist 的占位文件。
	rm -rf web/dist/*
