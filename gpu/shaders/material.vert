#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

layout(binding = 0) uniform Block {
	vec2 scale;
	vec2 pos;
} _block;

layout(location = 0) in vec2 pos;
layout(location = 1) in vec2 uv;

layout(location = 0) out vec2 vUV;

void main() {
	vUV = uv;
	gl_Position = vec4(pos*_block.scale + _block.pos, 0, 1);
}
