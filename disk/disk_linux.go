//go:build linux

package disk

import "fmt"

func partitions() ([]PartitionStat, error) {
	return nil, fmt.Errorf("not support")
}
