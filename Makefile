build: mkdir_bin build_mac build_linux build_win

build_mac:
	GOOS=darwin GOARCH=amd64 go build -o bin/influencer-for-mac

build_linux:
	GOOS=linux GOARCH=amd64 go build -o bin/influencer-for-linux

build_win:
	GOOS=windows GOARCH=386 go build -o bin/influencer-for-win.exe

mkdir_bin:
	mkdir -p ./bin
