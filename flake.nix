{
  description = "AX206 USB Sensor Display for NixOS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      # Systems supported
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];
      
      # Helper function to generate outputs for each system
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      
      # Nixpkgs instantiated for each system
      nixpkgsFor = forAllSystems (system: import nixpkgs {
        inherit system;
        config.allowUnfree = true; # For NVIDIA drivers
      });
    in
    {
      # Package definitions
      packages = forAllSystems (system: let
        pkgs = nixpkgsFor.${system};
        python = pkgs.python312;
      in {
        default = self.packages.${system}.axdisplay;
        
        axdisplay = python.pkgs.buildPythonApplication {
          pname = "axdisplay";
          version = "1.0.0";
          
          src = ./.;
          
          format = "pyproject";
          
          nativeBuildInputs = with python.pkgs; [
            setuptools
          ];
          
          propagatedBuildInputs = with python.pkgs; [
            pyusb
            pillow
          ];
          
          # Runtime dependencies
          buildInputs = with pkgs; [
            libusb1
          ];
          
          # Make sure we can find libusb at runtime
          makeWrapperArgs = [
            "--prefix" "LD_LIBRARY_PATH" ":" "${pkgs.libusb1}/lib"
          ];
          
          # Skip tests during build (no device available)
          doCheck = false;
          
          meta = with pkgs.lib; {
            description = "AX206 USB sensor display for NixOS";
            homepage = "https://github.com/your-username/sensorpanel";
            license = licenses.mit;
            platforms = platforms.linux;
            mainProgram = "axdisplay";
          };
        };
      });

      # Development shell
      devShells = forAllSystems (system: let
        pkgs = nixpkgsFor.${system};
        python = pkgs.python312;
      in {
        default = pkgs.mkShell {
          name = "axdisplay-dev";
          
          buildInputs = with pkgs; [
            # Python with packages
            (python.withPackages (ps: with ps; [
              pyusb
              pillow
              pytest
              black
              mypy
            ]))
            
            # USB tools
            libusb1
            usbutils
            
            # System tools
            lm_sensors
          ];
          
          shellHook = ''
            echo "AX206 Sensor Display Development Shell"
            echo ""
            echo "Commands:"
            echo "  python -m axdisplay --help     Show help"
            echo "  python -m axdisplay --list     List devices"
            echo "  python -m axdisplay --test     Display test pattern"
            echo "  python -m axdisplay            Run daemon"
            echo ""
            echo "Environment:"
            echo "  AXDISPLAY_INTERVAL=2           Update interval (seconds)"
            echo "  AXDISPLAY_THEME=dark           Theme (dark/light)"
            echo ""
          '';
        };
      });

      # NixOS module
      nixosModules = {
        default = self.nixosModules.ax206-display;
        
        ax206-display = { config, lib, pkgs, ... }:
          let
            cfg = config.services.ax206Display;
            package = self.packages.${pkgs.system}.axdisplay;
          in {
            options.services.ax206Display = {
              enable = lib.mkEnableOption "AX206 USB sensor display service";
              
              package = lib.mkOption {
                type = lib.types.package;
                default = package;
                description = "The axdisplay package to use";
              };
              
              interval = lib.mkOption {
                type = lib.types.str;
                default = "2";
                description = "Update interval in seconds";
              };
              
              theme = lib.mkOption {
                type = lib.types.enum [ "dark" "light" ];
                default = "dark";
                description = "Dashboard theme";
              };
              
              rotation = lib.mkOption {
                type = lib.types.enum [ 0 90 180 270 ];
                default = 0;
                description = "Display rotation in degrees";
              };
              
              gpu = {
                enable = lib.mkOption {
                  type = lib.types.bool;
                  default = true;
                  description = "Enable GPU monitoring";
                };
                
                method = lib.mkOption {
                  type = lib.types.enum [ "nvidia" "amd" "auto" "none" ];
                  default = "nvidia";
                  description = "GPU monitoring method";
                };
              };
              
              metrics = {
                cpu = lib.mkOption {
                  type = lib.types.bool;
                  default = true;
                  description = "Show CPU metrics";
                };
                
                ram = lib.mkOption {
                  type = lib.types.bool;
                  default = true;
                  description = "Show RAM metrics";
                };
                
                disk = lib.mkOption {
                  type = lib.types.bool;
                  default = true;
                  description = "Show disk metrics";
                };
                
                network = lib.mkOption {
                  type = lib.types.bool;
                  default = true;
                  description = "Show network metrics";
                };
              };
              
              diskMounts = lib.mkOption {
                type = lib.types.listOf lib.types.str;
                default = [ "/" ];
                description = "Disk mount points to monitor";
              };
              
              networkInterface = lib.mkOption {
                type = lib.types.str;
                default = "*";
                description = "Network interface pattern (e.g., 'enp*', 'eth0', '*' for auto)";
              };
              
              user = lib.mkOption {
                type = lib.types.str;
                default = "axdisplay";
                description = "User to run the service as";
              };
              
              group = lib.mkOption {
                type = lib.types.str;
                default = "axdisplay";
                description = "Group for USB device access";
              };
            };
            
            config = lib.mkIf cfg.enable {
              # Create dedicated user and group
              users.groups.${cfg.group} = {};
              users.users.${cfg.user} = {
                isSystemUser = true;
                group = cfg.group;
                extraGroups = [ "video" ]; # For GPU access
                description = "AX206 display service user";
              };
              
              # udev rules for USB device access
              services.udev.extraRules = ''
                # AX206 Digital Photo Frame - normal mode
                SUBSYSTEM=="usb", ATTR{idVendor}=="1908", ATTR{idProduct}=="0102", MODE="0660", GROUP="${cfg.group}", TAG+="uaccess"
                # AX206 Digital Photo Frame - bootloader mode
                SUBSYSTEM=="usb", ATTR{idVendor}=="1908", ATTR{idProduct}=="3318", MODE="0660", GROUP="${cfg.group}", TAG+="uaccess"
              '';
              
              # Systemd service
              systemd.services.ax206-display = {
                description = "AX206 USB Sensor Display";
                
                after = [ "network.target" "display-manager.service" ];
                wantedBy = [ "multi-user.target" ];
                
                # Wait for USB device to be ready after boot
                startLimitIntervalSec = 300;
                startLimitBurst = 10;
                
                environment = {
                  AXDISPLAY_INTERVAL = cfg.interval;
                  AXDISPLAY_THEME = cfg.theme;
                  AXDISPLAY_ROTATION = toString cfg.rotation;
                  AXDISPLAY_SHOW_CPU = if cfg.metrics.cpu then "1" else "0";
                  AXDISPLAY_SHOW_GPU = if cfg.gpu.enable then "1" else "0";
                  AXDISPLAY_SHOW_RAM = if cfg.metrics.ram then "1" else "0";
                  AXDISPLAY_SHOW_DISK = if cfg.metrics.disk then "1" else "0";
                  AXDISPLAY_SHOW_NETWORK = if cfg.metrics.network then "1" else "0";
                  AXDISPLAY_GPU_METHOD = cfg.gpu.method;
                  AXDISPLAY_DISK_MOUNTS = lib.concatStringsSep "," cfg.diskMounts;
                  AXDISPLAY_NETWORK_IF = cfg.networkInterface;
                };
                
                serviceConfig = {
                  Type = "simple";
                  User = cfg.user;
                  Group = cfg.group;
                  ExecStart = "${cfg.package}/bin/axdisplay";
                  Restart = "on-failure";
                  RestartSec = "5s";
                  
                  # Security hardening
                  ProtectSystem = "strict";
                  ProtectHome = true;
                  PrivateTmp = true;
                  NoNewPrivileges = true;
                  ProtectKernelTunables = true;
                  ProtectKernelModules = true;
                  ProtectControlGroups = true;
                  RestrictRealtime = true;
                  RestrictSUIDSGID = true;
                  
                  # Required capabilities
                  CapabilityBoundingSet = "";
                  AmbientCapabilities = "";
                  
                  # Allow USB access
                  DeviceAllow = [
                    "char-usb_device rwm"
                    "/dev/bus/usb/* rwm"
                  ];
                  SupplementaryGroups = [ cfg.group "video" ];
                  
                  # Logging
                  StandardOutput = "journal";
                  StandardError = "journal";
                  SyslogIdentifier = "ax206-display";
                };
                
                # Path to nvidia-smi (if using NVIDIA)
                path = lib.optional (cfg.gpu.method == "nvidia") 
                  config.hardware.nvidia.package.bin or [];
              };
            };
          };
      };

      # Overlay for including in other flakes
      overlays.default = final: prev: {
        axdisplay = self.packages.${prev.system}.axdisplay;
      };
    };
}
