.bindata:
	go-bindata ${GO_BINDATA_FLAGS} -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/*
	gofmt -w -s statics/bindata.go

builddep:
	go get github.com/jteeuwen/go-bindata/...

govendor:
	go get github.com/kardianos/govendor
	${GOPATH}/bin/govendor sync
