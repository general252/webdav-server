package disk

type PartitionStat struct {
	Device     string
	MountPoint string
	FsType     string
	Opts       string
}

func Partitions() ([]PartitionStat, error) {
	return partitions()
}
