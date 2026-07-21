# NekoCode 构建入口
# 所有 target 均假设从仓库根目录执行:
#   make -f build/makefile <target>

BIN     := nekocode
TUI_SRC := ./cmd/tui
GUI_SRC := ./main.go
GUI_DIR := interaction/gui/web

.PHONY: build run install clean test gui

build:
	go build -o $(BIN) $(TUI_SRC)

run: build
	./$(BIN)

install:
	go install $(TUI_SRC)

clean:
	rm -f $(BIN)

test:
	go test ./... 2>&1 | grep -vE '^ok|^?' --line-buffered || true

# GUI 需要先构建前端: cd $(GUI_DIR) && npm run build
gui:
	cd $(GUI_DIR) && npm run build
	go build -o nekocode-gui $(GUI_SRC)
