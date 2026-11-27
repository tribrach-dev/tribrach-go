module example.com/parking-services

go 1.24.0

replace github.com/tribrach-dev/tribrach-go => ../..

require github.com/tribrach-dev/tribrach-go v0.0.0

require (
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
)

tool github.com/tribrach-dev/tribrach-go/cmd/tribrach-go
