package plc

import (
	"fmt"
	"math"

	"github.com/future-architect/go-mcprotocol/mcp"
)

// ReadData reads data from the PLC for the specified device.
func ReadData(deviceType string, deviceNumber uint16, numberRegisters uint16, plcHost string, plcPort int) (interface{}, error) {
	// Connect to the PLC with MC protocol
	client, err := mcp.New3EClient(plcHost, plcPort, mcp.NewLocalStation())
	if err != nil {
		return nil, err
	}

	// Read data from the PLC
	data, err := client.Read(deviceType, int64(deviceNumber), int64(numberRegisters))
	if err != nil {
		return nil, err
	}

	var value interface{}
	if numberRegisters == 1 { // 16-bit device
		// Parse 16-bit data
		registerBinary, _ := mcp.NewParser().Do(data)
		data = registerBinary.Payload
		var val uint16
		for i := 0; i < len(data); i++ {
			val |= uint16(data[i]) << uint(8*i)
		}
		value = val
	} else if numberRegisters == 2 { // 32-bit device
		// Parse 32-bit data
		var val uint32
		registerBinary, _ := mcp.NewParser().Do(data)
		data = registerBinary.Payload
		for i := 0; i < len(data); i++ {
			val |= uint32(data[i]) << uint(8*i)
		}
		floatValue := math.Float32frombits(val)
		floatString := fmt.Sprintf("%.6f", floatValue)
		firstSixDigits := ""
		numDigits := 0
		for _, c := range floatString {
			if c == '-' || c == '.' {
				// Include minus sign and decimal point
				firstSixDigits += string(c)
			} else if numDigits < 6 {
				// Only include the first 6 digits
				firstSixDigits += string(c)
				numDigits++
			}
		}
		value = firstSixDigits
	} else {
		// Invalid number of registers
		return nil, fmt.Errorf("invalid number of registers: %d", numberRegisters)
	}

	return value, nil
}
