package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/experimental/devices/mcp9808"
	"periph.io/x/periph/host"

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

	b := bme280.Bme280{I2cbus: myi2c}
	s := sensors{myBme: &b}

	// Uncomment/comment next lines to suppress/increase verbosity of output
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	logger.ChangePackageLogLevel("bsbmp", logger.InfoLevel)

	http.HandleFunc("/", s.handler)
	log.Fatal(http.ListenAndServe(":80", nil))
	lg.Info("Starting mcp9808 reader...")
	foo()
}

func (s *sensors) handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, s.myBme.Read())
}

func foo() error {
	if _, err := host.Init(); err != nil {
		return err
	}
	address := flag.Int("address", 0x18, "I²C address")
	i2cbus := flag.String("bus", "", "I²C bus (/dev/i2c-1)")

	flag.Parse()

	lg.Info("Starting MCP9808 Temperature Sensor")
	if _, err := host.Init(); err != nil {
		return err
	}

	// Open default I²C bus.
	bus, err := i2creg.Open(*i2cbus)
	if err != nil {
		return fmt.Errorf("failed to open I²C: %v", err)
	}
	defer bus.Close()

	// Create a new temperature sensor a sense with default options.
	sensor, err := mcp9808.New(bus, &mcp9808.Opts{Addr: *address})
	if err != nil {
		return fmt.Errorf("failed to open new sensor: %v", err)
	}

	// Read values from sensor every second.
	everySecond := time.Tick(time.Second)
	var halt = make(chan os.Signal, 1)
	signal.Notify(halt, syscall.SIGTERM)
	signal.Notify(halt, syscall.SIGINT)

	lg.Info("ctrl+c to exit")
	for {
		select {
		case <-everySecond:
			t, err := sensor.SenseTemp()
			if err != nil {
				return fmt.Errorf("sensor reading error: %v", err)
			}
			lg.Info(t)

		case <-halt:
			return nil
		}
	}
}
