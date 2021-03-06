package charger

import (
	"fmt"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/modbus"
)

const (
	phLudwigRegEnConfig   = 4000  // Holding, Remanent
	phLudwigRegOutConfig  = 5500  // Holding, Remanent
	phLudwigRegEnable     = 20000 // Coil
	phLudwigRegOUT        = 23000 // Holding
	phLudwigRegMaxCurrent = 22000 // Holding
	phLudwigRegStatus     = 24000 // Input
)

// PhoenixLudwig is an api.ChargeController implementation for Phoenix EV-CC-AC1-M wallboxes.
// It uses Modbus TCP to communicate with the wallbox at modbus client id 255.
type PhoenixLudwig struct {
	log    *util.Logger
	conn   *modbus.Connection
	enMode string
}

func init() {
	registry.Add("phoenix-ludwig", NewPhoenixLudwigFromConfig)
}

//go:generate go run ../cmd/tools/decorate.go -p charger -f decoratePhoenixLudwig -b api.Charger -o phoenix-evcc_decorators -t "api.Meter,CurrentPower,func() (float64, error)" -t "api.MeterEnergy,TotalEnergy,func() (float64, error)" -t "api.MeterCurrent,Currents,func() (float64, float64, float64, error)"

// NewPhoenixLudwigFromConfig creates a Phoenix charger from generic config
func NewPhoenixLudwigFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := struct {
		modbus.Settings `mapstructure:",squash"`
	}{
		Settings: modbus.Settings{
			ID: 255,
		},
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	return NewPhoenixLudwig(cc.URI, cc.Device, cc.Comset, cc.Baudrate, cc.ID)
}

// NewPhoenixLudwig creates a Phoenix charger
func NewPhoenixLudwig(uri, device, comset string, baudrate int, id uint8) (*PhoenixLudwig, error) {
	conn, err := modbus.NewConnection(uri, device, comset, baudrate, true, id)
	if err != nil {
		return nil, err
	}
	var enMode string
	//	enMode = "modbus"
	log := util.NewLogger("evcc")
	conn.Logger(log.TRACE)

	wb := &PhoenixLudwig{
		log:    log,
		conn:   conn,
		enMode: enMode,
	}

//	wb.conn.WriteSingleRegister(phLudwigRegEnConfig, 1)  //write to registers for debugging
//	wb.conn.WriteSingleRegister(phLudwigRegOutConfig, 0) //write to registers for debugging

	RegEnConfig, err := wb.conn.ReadInputRegisters(phLudwigRegEnConfig, 1)
	if err != nil {
		return nil, err
	}
	wb.log.TRACE.Printf("Register 4000 is %d", RegEnConfig[1])

	RegOutConfig, err := wb.conn.ReadInputRegisters(phLudwigRegOutConfig, 1)
	if err != nil {
		return nil, err
	}
	wb.log.TRACE.Printf("Register 5500 is %d", RegOutConfig[1])

	if RegEnConfig[1] == 1 && RegOutConfig[1] == 0 {

		enMode = "out-pin"
		wb.log.TRACE.Printf("enMode: %s; Charger is enabled by switching the digital OUT on.", enMode)

	} else if RegEnConfig[1] == 3 {
		enMode = "modbus"
		wb.log.TRACE.Printf("enMode: %s; Charger is enabled by modbus.", enMode)

	} else {
		wb.log.ERROR.Printf("Registers 4000 = %d (and 5500 = %d) of Phoenix evcc are not configured for remote enabling", RegEnConfig[1], RegOutConfig[1])
	}

	wb.enMode = enMode

	return wb, nil
}

// Status implements the Charger.Status interface
func (wb *PhoenixLudwig) Status() (api.ChargeStatus, error) {
	b, err := wb.conn.ReadInputRegisters(phLudwigRegStatus, 1)
	if err != nil {
		return api.StatusNone, err
	}

	return api.ChargeStatus(string(b[0])), nil
}

// Enabled implements the Charger.Enabled interface
func (wb *PhoenixLudwig) Enabled() (bool, error) {
	var EN bool
	wb.log.TRACE.Printf("enMode: %s;", wb.enMode)

	d, err := wb.conn.ReadInputRegisters(phLudwigRegOUT, 1)
	if err != nil {
		return false, err
	}
	wb.log.TRACE.Printf("Register 23000 is %d)", d)

	b, err := wb.conn.ReadCoils(phLudwigRegEnable, 1)
	if err != nil {
		return false, err
	}
	wb.log.TRACE.Printf("Register 20000 is %d)", b)

	if wb.enMode == "out-pin" {
		EN = d[1] == 1
	} else {
		EN = b[0] == 1
	}
	wb.log.TRACE.Printf("enabled: %t", EN)

	return EN, nil
}

// Enable implements the Charger.Enable interface
func (wb *PhoenixLudwig) Enable(enable bool) error {
	var u uint16
	wb.log.TRACE.Printf("enMode: %s;", wb.enMode)

	wb.log.TRACE.Printf("enable: %t;", enable)

	if enable {
		u = 0xFF00
	}
	_, err := wb.conn.WriteSingleCoil(phLudwigRegEnable, u)
	if err != nil {
		return err
	}

	if wb.enMode == "out-pin" {
		if enable {
			u = 0x0001
		}
		//High-signal on pin OUT of the EV_CC_AC1-M  board (wire bridge between OUT and ENABLE necessary!!)
		_, err := wb.conn.WriteSingleRegister(phLudwigRegOUT, u)
		if err != nil {
			return err
		}

	}

	return err
}

// MaxCurrent implements the Charger.MaxCurrent interface
func (wb *PhoenixLudwig) MaxCurrent(current int64) error {
	if current < 6 {
		return fmt.Errorf("invalid current %d", current)
	}

	_, err := wb.conn.WriteSingleRegister(phLudwigRegMaxCurrent, uint16(current))

	return err
}
