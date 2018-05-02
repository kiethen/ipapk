# ipapk
ipa or apk parser written in golang, aims to extract app information

[![Build Status](https://travis-ci.org/phinexdaz/ipapk.svg?branch=master)](https://travis-ci.org/phinexdaz/ipapk)

## INSTALL
	$ go get github.com/phinexdaz/ipapk
  
## USAGE
```go
package main

import (
	"fmt"
	"github.com/phinexdaz/ipapk"
)

func main() {
	apk, _ := ipapk.NewAppParser("test.apk")
	fmt.Println(apk)
}
```
