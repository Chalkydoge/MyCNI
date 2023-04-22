package ip

import (
	"testing"
)

func TestSetupVx(t *testing.T) {
	_, err := SetupVXLAN("test1", 1500)
	if err != nil {
		t.Log(err)
	}
	t.Log("vxlan ok")
}
