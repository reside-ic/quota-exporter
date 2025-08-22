{
  perSystem = { self', pkgs, lib, ... }: {
    packages.vm-test = (pkgs.testers.runNixOSTest {
      name = "vm-test";
      nodes.machine = { config, pkgs, ... }: {
        system.stateVersion = "25.05";
        systemd.services.quota-exporter = {
          wantedBy = [ "multi-user.target" ];
          serviceConfig = {
            ExecStart = "${lib.getExe self'.packages.quota-exporter} --mountpoint /data";
          };
        };
        users.users.alice = { isNormalUser = true; };
        users.users.bob = { isNormalUser = true; };

        environment.systemPackages = [ pkgs.linuxquota ];

        virtualisation.emptyDiskImages = [ 1 ]; # size in MB of the disk

        boot.initrd.postDeviceCommands = ''
          ${pkgs.e2fsprogs}/bin/mkfs.ext4 -O quota -L data /dev/vdb
        '';

        virtualisation.fileSystems."/data" = {
          device = "/dev/vdb";
          fsType = "ext4";
          options = [ "usrquota" ];
        };
      };

      extraPythonPackages = ps: [ ps.prometheus_client ];
      testScript = ''
        from prometheus_client.parser import text_string_to_metric_families

        def get(key):
          data = machine.succeed("curl -sf http://localhost:10018/metrics")
          metrics = { m.name: m.samples for m in text_string_to_metric_families(data) }
          return { s.labels['user']: s.value for s in metrics[key] }

        machine.wait_for_unit("multi-user.target")
        machine.succeed("chmod 777 /data")

        usage = get('quota_user_space_used_bytes')
        assert set(usage.keys()) == {"root"}

        machine.succeed("su -- alice -c 'dd if=/dev/random of=/data/foo bs=1024 count=1'")
        machine.succeed("su -- bob -c 'dd if=/dev/random of=/data/bar bs=1024 count=512'")

        # Alice uses 1KB, Bob uses 512KB. Allow for a bit of overhead just
        # in case.
        usage = get('quota_user_space_used_bytes')
        assert 1024 <= usage['alice'] < 2048
        assert (512 * 1024) <= usage['bob'] < (513*1024)

        machine.succeed("setquota -u alice 5K 1M 0 0 /data ")

        soft_limits = get('quota_user_space_soft_limit_bytes')
        hard_limits = get('quota_user_space_hard_limit_bytes')
        assert soft_limits['alice'] == 5 * 1024
        assert hard_limits['alice'] == 1024 * 1024

        data = machine.succeed("curl -sf http://localhost:10018/metrics")
        metrics = { m.name: m.samples for m in text_string_to_metric_families(data) }
        # 7 days is the Linux default for these
        assert metrics['quota_user_space_grace_period_seconds'][0].value == 7*24*3600
        assert metrics['quota_user_inodes_grace_period_seconds'][0].value == 7*24*3600

        machine.succeed("setquota -t 3600 7200 /data")
        data = machine.succeed("curl -sf http://localhost:10018/metrics")
        metrics = { m.name: m.samples for m in text_string_to_metric_families(data) }
        assert metrics['quota_user_space_grace_period_seconds'][0].value == 3600
        assert metrics['quota_user_inodes_grace_period_seconds'][0].value == 7200
      '';
    }).driver;
  };
}
