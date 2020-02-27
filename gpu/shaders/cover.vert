#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

#include <common.inc>

layout(binding = 0) uniform Block {
	float z;
	vec2 scale;
	vec2 offset;
	vec2 uvCoverScale;
	vec2 uvCoverOffset;
	vec2 uvScale;
	vec2 uvOffset;
};

layout(location = 0) in vec2 pos;

layout(location = 0) out vec2 vCoverUV;

layout(location = 1) in vec2 uv;
layout(location = 1) out vec2 vUV;

void main() {
    gl_Position = vec4(pos*scale + offset, z, 1);
	vUV = uv*uvScale + uvOffset;
	vec3[2] fboTrans = fboTextureTransform();
	vec3 uv3 = transform3x2(fboTrans, vec3(uv, 1.0));
	vCoverUV = (uv3*vec3(uvCoverScale, 1.0)+vec3(uvCoverOffset, 0.0)).xy;
}
