{
  description = "SensorPanel - USB Display for System Metrics";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };
      in {
        packages = {
          default = self.packages.${system}.sensorpanel;
          
          sensorpanel = pkgs.buildGoModule {
            pname = "sensorpanel";
            version = "1.0.0";
            
            src = ./.;
            
            vendorHash = null; # Set to null for initial build, update after first build
            
            # Runtime dependencies
            buildInputs = with pkgs; [
              libusb1
            ];
            
            nativeBuildInputs = with pkgs; [
              pkg-config
            ];
            
            # CGO is needed for gousb
            CGO_ENABLED = "1";
            
            ldflags = [ "-s" "-w" ];
            
            meta = with pkgs.lib; {
              description = "USB sensor display for system metrics";
              homepage = "https://github.com/your-username/sensorpanel";
              license = licenses.mit;
              platforms = platforms.linux ++ platforms.darwin;
              mainProgram = "sensorpanel";
            };
          };
        };

        devShells.default = pkgs.mkShell {
          name = "sensorpanel-dev";
          
          buildInputs = with pkgs; [
            # Go
            go
            gopls
            gotools
            go-tools
            
            # USB
            libusb1
            pkg-config
            usbutils
            
            # System monitoring
            lm_sensors
            
            # For theme development
            nodejs
          ];
          
          shellHook = ''
            echo "SensorPanel Development Shell"
            echo ""
            echo "Build:    go build ."
            echo "Run:      ./sensorpanel run"
            echo "Help:     ./sensorpanel --help"
            echo ""
          '';
        };
      }
    ) // {
      # NixOS module
      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.sensorpanel;
        in {
          options.services.sensorpanel = {
            enable = lib.mkEnableOption "SensorPanel USB display service";
            
            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.sensorpanel;
              description = "The sensorpanel package to use";
            };
            
            interval = lib.mkOption {
              type = lib.types.float;
              default = 1.0;
              description = "Update interval in seconds";
            };
            
            brightness = lib.mkOption {
              type = lib.types.ints.between 0 7;
              default = 7;
              description = "Display brightness (0-7)";
            };
            
            theme = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = "Theme name (null for built-in renderer)";
            };
            
            diskMounts = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ "/" ];
              description = "Disk mount points to monitor";
            };
            
            user = lib.mkOption {
              type = lib.types.str;
              default = "sensorpanel";
              description = "User to run the service as";
            };
            
            group = lib.mkOption {
              type = lib.types.str;
              default = "sensorpanel";
              description = "Group for USB device access";
            };
          };
          
          config = lib.mkIf cfg.enable {
            # Create dedicated user and group
            users.groups.${cfg.group} = {};
            users.users.${cfg.user} = {
              isSystemUser = true;
              group = cfg.group;
              extraGroups = [ "video" ];
              description = "SensorPanel service user";
              home = "/var/lib/sensorpanel";
              createHome = true;
            };
            
            # udev rules for USB device access
            services.udev.extraRules = ''
              # AX206 USB Display
              SUBSYSTEM=="usb", ATTR{idVendor}=="1908", ATTR{idProduct}=="0102", MODE="0660", GROUP="${cfg.group}", TAG+="uaccess"
            '';
            
            # Systemd service
            systemd.services.sensorpanel = {
              description = "SensorPanel USB Display Service";
              after = [ "network.target" ];
              wantedBy = [ "multi-user.target" ];
              
              serviceConfig = {
                Type = "simple";
                User = cfg.user;
                Group = cfg.group;
                ExecStart = "${cfg.package}/bin/sensorpanel run --interval ${toString cfg.interval} --brightness ${toString cfg.brightness}";
                Restart = "on-failure";
                RestartSec = "5s";
                
                # Security hardening
                ProtectSystem = "strict";
                ProtectHome = "read-only";
                PrivateTmp = true;
                NoNewPrivileges = true;
                
                # Allow USB access
                DeviceAllow = [ "char-usb_device rwm" ];
                SupplementaryGroups = [ cfg.group "video" ];
              };
            };
          };
        };
    };
}
