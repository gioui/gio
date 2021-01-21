#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision mediump float;

{{.Header}}

// Use high precision to be pixel accurate for
// large cover atlases.
layout(location = 0) in highp vec2 vCoverUV;
layout(location = 1) in vec2 vUV;

layout(binding = 1) uniform sampler2D cover;

layout(location = 0) out vec4 fragColor;

void main() {
    fragColor = {{.FetchColorExpr}};
	float cover = min(abs(texture(cover, vCoverUV).r), 1.0);
	fragColor *= cover;
}
