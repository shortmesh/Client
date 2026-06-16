package devices

import (
	"testing"
)

func TestDeviceListExtracted(t *testing.T) {
	message := "* `123456789` (+123456789) - `CONNECTED`"

	expectedDevice := "123456789"

	outputs, err := ParseListDevices(message)
	if err != nil {
		t.Errorf("Error %s", err)
		return
	}

	if expectedDevice != outputs[0].Device {
		t.Errorf("wanted %s, got %s", expectedDevice, outputs[0].Device)
		return
	}
	if !outputs[0].Connected {
		t.Errorf("status is false")
	}

	message = "* `521ec40f-d2ed-4649-a912-c0533bb37274` (+237696637975) - `CONNECTED`"
	expectedDevice = "521ec40f-d2ed-4649-a912-c0533bb37274"
	outputs, err = ParseListDevices(message)
	if err != nil {
		t.Errorf("Error %s", err)
		return
	}

	if expectedDevice != outputs[0].Device {
		t.Errorf("wanted %s, got %s", expectedDevice, outputs[0].Device)
		return
	}
	if !outputs[0].Connected {
		t.Errorf("status is false")
	}
}
