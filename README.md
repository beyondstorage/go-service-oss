# go-services-oss

[Aliyun Object Storage](https://cn.aliyun.com/product/oss) service support for [go-storage](https://github.com/beyondstorage/go-storage).

## Notes

**This package has been moved to [go-storage](https://github.com/beyondstorage/go-storage/tree/master/services/oss).**

```shell
go get go.beyondstorage.io/services/oss/v3
```

## Install

```go
go get github.com/beyondstorage/go-service-oss/v2
```

## Usage

```go
import (
	"log"

	_ "github.com/beyondstorage/go-service-oss/v2"
	"github.com/beyondstorage/go-storage/v4/services"
)

func main() {
	store, err := services.NewStoragerFromString("oss://bucket_name/path/to/workdir?credential=hmac:<access_key>:<secret_key>&endpoint=https:<location>.aliyuncs.com")
	if err != nil {
		log.Fatal(err)
	}

	// Write data from io.Reader into hello.txt
	n, err := store.Write("hello.txt", r, length)
}
```

- See more examples in [go-storage-example](https://github.com/beyondstorage/go-storage-example).
- Read [more docs](https://beyondstorage.io/docs/go-storage/services/oss) about go-service-oss. 
