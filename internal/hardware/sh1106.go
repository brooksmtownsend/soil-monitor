package hardware

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"

	"soilmonitor/internal/core"
	"soilmonitor/internal/display"
)

const sh1106ColumnOffset = 2

type SH1106 struct {
	mu     sync.Mutex
	port   spi.PortCloser
	conn   spi.Conn
	dc     gpio.PinIO
	reset  gpio.PinIO
	closed bool
}

func NewSH1106(portName string) (*SH1106, error) {
	port, err := spireg.Open(portName)
	if err != nil {
		return nil, fmt.Errorf("open SPI port %s: %w", portName, err)
	}
	conn, err := port.Connect(10*physic.MegaHertz, spi.Mode0, 8)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("configure SPI port %s: %w", portName, err)
	}
	dc := gpioreg.ByName("GPIO24")
	reset := gpioreg.ByName("GPIO25")
	if dc == nil || reset == nil {
		_ = port.Close()
		return nil, fmt.Errorf("display control pins GPIO24 and GPIO25 must be available")
	}
	if err := dc.Out(gpio.Low); err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("configure display DC pin: %w", err)
	}
	if err := reset.Out(gpio.High); err != nil {
		_ = dc.Halt()
		_ = port.Close()
		return nil, fmt.Errorf("configure display reset pin: %w", err)
	}
	displayAdapter := &SH1106{port: port, conn: conn, dc: dc, reset: reset}
	if err := displayAdapter.initialize(); err != nil {
		_ = displayAdapter.Close()
		return nil, fmt.Errorf("initialize SH1106 display: %w", err)
	}
	return displayAdapter, nil
}

func (d *SH1106) Render(view core.View) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return fmt.Errorf("display is closed")
	}
	return d.writeFrame(display.Frame(view))
}

func (d *SH1106) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	return errors.Join(
		d.command(0xAE),
		d.dc.Halt(),
		d.reset.Halt(),
		d.port.Close(),
	)
}

func (d *SH1106) initialize() error {
	if err := d.reset.Out(gpio.Low); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	if err := d.reset.Out(gpio.High); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return d.command(
		0xAE,
		0xD5, 0x80,
		0xA8, 0x3F,
		0xD3, 0x00,
		0x40,
		0xAD, 0x8B,
		0xA1,
		0xC8,
		0xDA, 0x12,
		0x81, 0x7F,
		0xD9, 0x22,
		0xDB, 0x40,
		0xA4,
		0xA6,
		0xAF,
	)
}

func (d *SH1106) command(commands ...byte) error {
	if err := d.dc.Out(gpio.Low); err != nil {
		return err
	}
	return d.conn.Tx(commands, nil)
}

func (d *SH1106) writeFrame(frame image.Image) error {
	bounds := frame.Bounds()
	if bounds.Dx() != display.Width || bounds.Dy() != display.Height {
		return fmt.Errorf("frame must be %dx%d pixels", display.Width, display.Height)
	}
	pageData := make([]byte, display.Width)
	for page := 0; page < display.Height/8; page++ {
		if err := d.command(byte(0xB0+page), byte(sh1106ColumnOffset&0x0F), byte(0x10|(sh1106ColumnOffset>>4))); err != nil {
			return fmt.Errorf("select page %d: %w", page, err)
		}
		encodePage(pageData, frame, page)
		if err := d.dc.Out(gpio.High); err != nil {
			return err
		}
		if err := d.conn.Tx(pageData, nil); err != nil {
			return fmt.Errorf("write page %d: %w", page, err)
		}
	}
	return nil
}

func encodePage(destination []byte, frame image.Image, page int) {
	bounds := frame.Bounds()
	for x := range destination {
		var column byte
		for bit := 0; bit < 8; bit++ {
			_, _, value, _ := frame.At(bounds.Min.X+x, bounds.Min.Y+page*8+bit).RGBA()
			if value > 0x7FFF {
				column |= 1 << bit
			}
		}
		destination[x] = column
	}
}
