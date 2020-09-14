package main

import (
	"fmt"
	"log"
	"net/http"

	"./bme280"

	"github.com/d2r2/go-i2c"
	logger "github.com/d2r2/go-logger"
)

var lg = logger.NewPackageLogger("main",
	logger.DebugLevel,
	// logger.InfoLevel,
)

type sensors struct {
	myBme *bme280.Bme280
}

func main() {
	defer logger.FinalizeLogger()
	// Create new connection to i2c-bus on 1 line with address 0x76.
	// Use i2cdetect utility to find device address over the i2c-bus
	myi2c, err := i2c.NewI2C(0x76, 1)
	if err != nil {
		lg.Fatal(err)
	}
	defer myi2c.Close()

	b := bme280.Bme280{i2cbus: myi2c}

	// Uncomment/comment next lines to suppress/increase verbosity of output
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("bsbmp", logger.InfoLevel)

	http.HandleFunc("/", b.handler)
	log.Fatal(http.ListenAndServe(":80", nil))
}

func (s *sensors) handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, s.myBme.Read())
}
