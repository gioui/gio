#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision mediump float;

// Use high precision to be pixel accurate for
// large cover atlases.
layout(location = 0) in highp vec2 vUV;

layout(binding = 0) uniform sampler2D cover;

layout(location = 0) out vec4 fragColor;

void main() {
  float cover = abs(texture(cover, vUV).r);
  fragColor.r = cover;
}
