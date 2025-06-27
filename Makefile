
all: win lin

win:
	GOOS=windows GOARCH=amd64 go build -ldflags="-w -s" -o webdav_win_amd64.exe .

lin:
	GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o webdav_lin_amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o webdav_lin_arm64 .
