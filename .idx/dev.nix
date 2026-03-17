{ pkgs, ... }: {
  packages = [
    pkgs.docker
  ];
  services.docker = {
    enable = true;
  };
}