package main

import (
	"log"
	"os/user"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

type QuotaCollector struct {
	mountpoints []string

	userSpaceUsed        *prometheus.Desc
	userSpaceHardLimit   *prometheus.Desc
	userSpaceSoftLimit   *prometheus.Desc
	userSpaceGracePeriod *prometheus.Desc

	userInodesUsed        *prometheus.Desc
	userInodesHardLimit   *prometheus.Desc
	userInodesSoftLimit   *prometheus.Desc
	userInodesGracePeriod *prometheus.Desc
}

func NewQuotaCollector(mountpoints []string) *QuotaCollector {
	userLabels := []string{"mountpoint", "user"}
	return &QuotaCollector{
		mountpoints: mountpoints,

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

func (c *QuotaCollector) Collect(ch chan<- prometheus.Metric) {
	for _, path := range c.mountpoints {
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
