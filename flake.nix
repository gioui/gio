# SPDX-License-Identifier: Unlicense OR MIT
{
  description = "Gio build environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, utils }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;

          # allow unfree Android packages.
          config.allowUnfree = true;
          # accept the Android SDK license.
          config.android_sdk.accept_license = true; 
        };
      in {
        devShells = let
          android-sdk = let
            androidComposition = pkgs.androidenv.composeAndroidPackages {
              platformVersions = [ "latest" ];
              abiVersions = [ "armeabi-v7a" "arm64-v8a" ];
               # Omit the deprecated tools package.
              toolsVersion = null;
              includeNDK = true;
            };
          in androidComposition.androidsdk;
        in {
          default = with pkgs;
            mkShell (rec {
              ANDROID_HOME = "${android-sdk}/libexec/android-sdk";
              packages = [ android-sdk jdk clang ]
                ++ (if stdenv.isLinux then [
                  vulkan-headers
                  libxkbcommon
                  wayland
                  xorg.libX11
                  xorg.libXcursor
                  xorg.libXfixes
                  libGL
                  pkg-config
                ] else
                  [ ]);
            } // (if stdenv.isLinux then {
              LD_LIBRARY_PATH = "${vulkan-loader}/lib";
            } else
              { }));
        };
      });
}
