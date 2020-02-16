#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

layout(binding = 0) uniform Block {
	float z;
	vec2 scale;
	vec2 offset;
	vec2 uvScale;
	vec2 uvOffset;
	vec2 uvCoverScale;
	vec2 uvCoverOffset;
} uniforms;

layout(location = 0) in vec2 pos;

layout(location = 0) out vec2 vCoverUV;

layout(location = 1) in vec2 uv;
layout(location = 1) out vec2 vUV;

void main() {
    gl_Position = vec4(pos*uniforms.scale + uniforms.offset, uniforms.z, 1);
	vUV = uv*uniforms.uvScale + uniforms.uvOffset;
	vCoverUV = uv*uniforms.uvCoverScale+uniforms.uvCoverOffset;
}
