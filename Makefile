.bindata:
	go-bindata ${GO_BINDATA_FLAGS} -nometadata -o statics/bindata.go -pkg=statics -ignore=bindata.go statics/*
	gofmt -w -s statics/bindata.go
