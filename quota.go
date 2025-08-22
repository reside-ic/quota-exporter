// This is inspired by https://anexia.com/blog/en/filesystem-quota-management-in-go/

package main

import (
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The constants and type definitions can be found in include/uapi/linux/quota.h.
// These are part of the Linux ABI and are guaranteed not to change.

const (
	Q_GETNEXTQUOTA int = 0x800009
	Q_SYNC         int = 0x800001
	Q_QUOTAON      int = 0x800002
	Q_QUOTAOFF     int = 0x800003
	Q_GETFMT       int = 0x800004
	Q_GETINFO      int = 0x800005
	Q_SETINFO      int = 0x800006
	Q_GETQUOTA     int = 0x800007
	Q_SETQUOTA     int = 0x800008
)

const (
	USRQUOTA int = 0
	GRPQUOTA int = 1
	PRJQUOTA int = 2
)

const SUBCMDMASK int = 0x00ff
const SUBCMDSHIFT int = 8

func QCMD(cmd int, type_ int) int {
	return (cmd << SUBCMDSHIFT) | (type_ & SUBCMDMASK)
}

const (
	QIF_DQBLKSIZE_BITS uint64 = 10
	QIF_DQBLKSIZE      uint64 = (1 << QIF_DQBLKSIZE_BITS)
	BlockSize          uint64 = QIF_DQBLKSIZE
)

type if_dqblk struct {
	dqb_bhardlimit uint64
	dqb_bsoftlimit uint64
	dqb_curspace   uint64
	dqb_ihardlimit uint64
	dqb_isoftlimit uint64
	dqb_curinodes  uint64
	dqb_btime      uint64
	dqb_itime      uint64
	dqb_valid      uint32
}

type if_nextdqblk struct {
	dqb_bhardlimit uint64
	dqb_bsoftlimit uint64
	dqb_curspace   uint64
	dqb_ihardlimit uint64
	dqb_isoftlimit uint64
	dqb_curinodes  uint64
	dqb_btime      uint64
	dqb_itime      uint64
	dqb_valid      uint32
	dqb_id         uint32
}

type if_dqinfo struct {
	dqi_bgrace uint64
	dqi_igrace uint64
	dqi_flags  uint32
	dqi_valid  uint32
}

type Quota struct {
	Id uint32

	BlockHardLimit uint64
	BlockSoftLimit uint64
	CurrentSpace   uint64

	InodeHardLimit uint64
	InodeSoftLimit uint64
	CurrentInodes  uint64

	BlockTimeLimit uint64
	InodeTimeLimit uint64
}

func quotactlFd(fd int, op int, id int, ptr unsafe.Pointer) error {
	_, _, e1 := unix.Syscall6(unix.SYS_QUOTACTL_FD, uintptr(fd), uintptr(op), uintptr(id), uintptr(ptr), 0, 0)
	if e1 != 0 {
		return e1
	} else {
		return nil
	}
}

func getQuotaFd(fd int, type_ int, id int, data *if_dqblk) error {
	return quotactlFd(fd, QCMD(Q_GETQUOTA, type_), id, unsafe.Pointer(data))
}

func getNextQuotaFd(fd int, type_ int, id int, data *if_nextdqblk) error {
	return quotactlFd(fd, QCMD(Q_GETNEXTQUOTA, type_), id, unsafe.Pointer(data))
}

func getInfoFd(fd int, type_ int, data *if_dqinfo) error {
	return quotactlFd(fd, QCMD(Q_GETINFO, type_), 0, unsafe.Pointer(data))
}

func GetQuota(path string, type_ int, id int) (Quota, error) {
	fd, err := unix.Open(path, unix.O_DIRECTORY|unix.O_PATH, 0)
	if err != nil {
		return Quota{}, err
	}
	defer unix.Close(fd)

	var data if_dqblk
	err = getQuotaFd(fd, type_, id, &data)
	if err != nil {
		return Quota{}, err
	}

	return Quota{
		Id: uint32(id),

		BlockHardLimit: data.dqb_bhardlimit,
		BlockSoftLimit: data.dqb_bsoftlimit,
		CurrentSpace:   data.dqb_curspace,

		InodeHardLimit: data.dqb_ihardlimit,
		InodeSoftLimit: data.dqb_isoftlimit,
		CurrentInodes:  data.dqb_curinodes,

		BlockTimeLimit: data.dqb_btime,
		InodeTimeLimit: data.dqb_itime,
	}, nil
}

func GetQuotas(path string, type_ int) ([]Quota, error) {
	fd, err := unix.Open(path, unix.O_DIRECTORY|unix.O_PATH, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	var result []Quota

	id := 0
	var data if_nextdqblk
	for {
		err = getNextQuotaFd(fd, type_, id, &data)
		if err == syscall.ENOENT {
			break
		}
		if err != nil {
			return nil, err
		}

		result = append(result, Quota{
			Id: data.dqb_id,

			BlockHardLimit: data.dqb_bhardlimit,
			BlockSoftLimit: data.dqb_bsoftlimit,
			CurrentSpace:   data.dqb_curspace,

			InodeHardLimit: data.dqb_ihardlimit,
			InodeSoftLimit: data.dqb_isoftlimit,
			CurrentInodes:  data.dqb_curinodes,

			BlockTimeLimit: data.dqb_btime,
			InodeTimeLimit: data.dqb_itime,
		})

		id = int(data.dqb_id) + 1
	}

	return result, nil
}

type QuotaInfo struct {
	BlockSoftLimitGracePeriod time.Duration
	InodeSoftLimitGracePeriod time.Duration
}

func GetQuotaInfo(path string, type_ int) (QuotaInfo, error) {
	fd, err := unix.Open(path, unix.O_DIRECTORY|unix.O_PATH, 0)
	if err != nil {
		return QuotaInfo{}, err
	}
	defer unix.Close(fd)

	var data if_dqinfo
	err = getInfoFd(fd, type_, &data)
	if err != nil {
		return QuotaInfo{}, err
	}

	return QuotaInfo{
		BlockSoftLimitGracePeriod: time.Duration(data.dqi_bgrace) * time.Second,
		InodeSoftLimitGracePeriod: time.Duration(data.dqi_igrace) * time.Second,
	}, nil
}
