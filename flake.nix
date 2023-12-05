# SPDX-License-Identifier: Unlicense OR MIT
{
  description = "Gio build environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/23.11";
    android.url = "github:tadfisher/android-nixpkgs";
    android.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, android }:
    let
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f system);
    in
    {
      devShells = forAllSystems
        (system:
          let
            pkgs = import nixpkgs {
              inherit system;
            };
            android-sdk = android.sdk.${system} (sdkPkgs: with sdkPkgs;
              [
                build-tools-31-0-0
                cmdline-tools-latest
                platform-tools
                platforms-android-31
                ndk-bundle
              ]);
          in
          {
            default = with pkgs; mkShell
              ({
                ANDROID_SDK_ROOT = "${android-sdk}/share/android-sdk";
                JAVA_HOME = jdk17.home;
                packages = [
                  android-sdk
                  jdk17
                  clang
                ] ++ (if stdenv.isLinux then [
                  vulkan-headers
                  libxkbcommon
                  wayland
                  xorg.libX11
                  xorg.libXcursor
                  xorg.libXfixes
                  libGL
                  pkg-config
                ] else if stdenv.isDarwin then [
                  darwin.apple_sdk_11_0.frameworks.Foundation
                  darwin.apple_sdk_11_0.frameworks.Metal
                  darwin.apple_sdk_11_0.frameworks.QuartzCore
                  darwin.apple_sdk_11_0.frameworks.AppKit
                  darwin.apple_sdk_11_0.MacOSX-SDK
                ] else [ ]);
              } // (if stdenv.isLinux then {
                LD_LIBRARY_PATH = "${vulkan-loader}/lib";
              } else { }));
          }
        );
    };
}
