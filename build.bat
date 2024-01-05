go build -o bin\enum-subdomain-go-windows-amd64.exe

SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64
go build -o bin\enum-subdomain-go-linux-amd64

SET CGO_ENABLED=0
SET GOOS=darwin
SET GOARCH=amd64
go build -o bin\enum-subdomain-go-darwin-amd64

SET CGO_ENABLED=0
SET GOOS=darwin
SET GOARCH=arm64
go build -o bin\enum-subdomain-go-darwin-arm64