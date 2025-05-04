{ pkgs ? import <nixpkgs> {} }:

(pkgs.buildFHSUserEnv {
  name = "ytviewer";
  targetPkgs = pkgs: with pkgs; [
    go
  ];
  profile = ''
    export GO111MODULE=on
  '';
}).env