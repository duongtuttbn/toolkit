package toolkit

import (
	"fmt"
	"testing"
)

func TestPool(t *testing.T) {
	data, err := ConvertType[string]("test")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(data)
}
