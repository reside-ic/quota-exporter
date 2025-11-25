package main

import (
	"log"
	"os/user"
	"strconv"

	"github.com/moby/sys/mountinfo"
	"github.com/prometheus/client_golang/prometheus"
)

type QuotaCollector struct {
	allMountpoints bool
	mountpoints    []string

	userSpaceUsed        *prometheus.Desc
	userSpaceHardLimit   *prometheus.Desc
	userSpaceSoftLimit   *prometheus.Desc
	userSpaceGracePeriod *prometheus.Desc

	userInodesUsed        *prometheus.Desc
	userInodesHardLimit   *prometheus.Desc
	userInodesSoftLimit   *prometheus.Desc
	userInodesGracePeriod *prometheus.Desc
}

func NewQuotaCollector(allMountpoints bool, mountpoints []string) *QuotaCollector {
	userLabels := []string{"mountpoint", "user"}
	return &QuotaCollector{
		allMountpoints: allMountpoints,
		mountpoints:    mountpoints,

		userSpaceUsed:        prometheus.NewDesc("quota_user_space_used_bytes", "Number of bytes currently occupied by a user", userLabels, nil),
		userSpaceHardLimit:   prometheus.NewDesc("quota_user_space_hard_limit_bytes", "Hard-limit for space usage for a user", userLabels, nil),
		userSpaceSoftLimit:   prometheus.NewDesc("quota_user_space_soft_limit_bytes", "Soft-limit for space usage for a user", userLabels, nil),
		userSpaceGracePeriod: prometheus.NewDesc("quota_user_space_grace_period_seconds", "Grace period for space usage soft limit", []string{"mountpoints"}, nil),

		userInodesUsed:        prometheus.NewDesc("quota_user_inodes_used_count", "Number of inodes in use by a user", userLabels, nil),
		userInodesHardLimit:   prometheus.NewDesc("quota_user_inodes_hard_limit_count", "Hard-limit for the number of inodes in use by a user", userLabels, nil),
		userInodesSoftLimit:   prometheus.NewDesc("quota_user_inodes_soft_limit_count", "Soft-limit for the number of inodes in use by a user", userLabels, nil),
		userInodesGracePeriod: prometheus.NewDesc("quota_user_inodes_grace_period_seconds", "Grace period for inodes usage soft limit", []string{"mountpoints"}, nil),
	}
}

func (c *QuotaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.userSpaceUsed
	ch <- c.userSpaceHardLimit
	ch <- c.userSpaceSoftLimit

	ch <- c.userInodesUsed
	ch <- c.userInodesHardLimit
	ch <- c.userInodesSoftLimit
}

func lookupUser(id int) string {
	uid := strconv.Itoa(id)
	user, err := user.LookupId(uid)
	if err == nil {
		return user.Username
	} else {
		return uid
	}
}

// List all mountpoints that have quotas enabled.
// We do this by looking for the usrquota mount option. This is similar to what quota-tools does.
func listQuotaMountpoints() ([]string, error) {
	mounts, err := mountinfo.GetMounts(func(info *mountinfo.Info) (skip, stop bool) {
		ok, err := QuotaIsEnabled(info, USRQUOTA)
		if err != nil {
			log.Printf("Error while checking if mountpoint '%s' supports quotas: %v", info.Mountpoint, err)
			return true, false
		} else {
			return !ok, false
		}
	})
	if err != nil {
		return nil, err
	}

	// This filters out bind mounts from the result.
	// For each (major, minor) pair of devices, it takes the first entry found in `/proc/self/mounts`.
	//
	// Inspired by the conversation at https: //github.com/prometheus/node_exporter/issues/600
	type Key struct{ major, minor int }
	found := make(map[Key]bool)
	var result []string
	for _, info := range mounts {
		key := Key{major: info.Major, minor: info.Minor}
		if !found[key] {
			result = append(result, info.Mountpoint)
			found[key] = true
		}
	}
	return result, nil
}

func (c *QuotaCollector) Collect(ch chan<- prometheus.Metric) {
	var err error
	mountpoints := c.mountpoints
	if c.allMountpoints {
		mountpoints, err = listQuotaMountpoints()
		if err != nil {
			log.Printf("Error while listing mountpoints %v", err)
			return
		}
	}

	for _, path := range mountpoints {
		info, err := GetQuotaInfo(path, USRQUOTA)
		if err != nil {
			log.Printf("Error while getting quota information for mountpoint %s: %v", path, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.userSpaceGracePeriod,
			prometheus.GaugeValue,
			float64(info.BlockSoftLimitGracePeriod.Seconds()),
			path,
		)
		ch <- prometheus.MustNewConstMetric(
			c.userInodesGracePeriod,
			prometheus.GaugeValue,
			float64(info.InodeSoftLimitGracePeriod.Seconds()),
			path,
		)

		quotas, err := GetQuotas(path, USRQUOTA)
		if err != nil {
			log.Printf("Error while collecting quotas for mountpoint %s: %v", path, err)
			continue
		}

		for _, q := range quotas {
			user := lookupUser(int(q.Id))

			ch <- prometheus.MustNewConstMetric(c.userSpaceUsed, prometheus.GaugeValue, float64(q.CurrentSpace), path, user)
			ch <- prometheus.MustNewConstMetric(c.userSpaceHardLimit, prometheus.GaugeValue, float64(q.BlockHardLimit*BlockSize), path, user)
			ch <- prometheus.MustNewConstMetric(c.userSpaceSoftLimit, prometheus.GaugeValue, float64(q.BlockSoftLimit*BlockSize), path, user)

			ch <- prometheus.MustNewConstMetric(c.userInodesUsed, prometheus.GaugeValue, float64(q.CurrentInodes), path, user)
			ch <- prometheus.MustNewConstMetric(c.userInodesHardLimit, prometheus.GaugeValue, float64(q.InodeHardLimit), path, user)
			ch <- prometheus.MustNewConstMetric(c.userInodesSoftLimit, prometheus.GaugeValue, float64(q.InodeSoftLimit), path, user)
		}
	}
}
