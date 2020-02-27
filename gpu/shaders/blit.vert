#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

#include <common.inc>

layout(binding = 0) uniform Block {
	vec4 transform;
	vec4 uvTransform;
	float z;
};

layout(location = 0) in vec2 pos;

layout(location = 1) in vec2 uv;

layout(location = 0) out vec2 vUV;

void main() {
	vec2 p = pos*transform.xy + transform.zw;
	gl_Position = toClipSpace(vec4(p, z, 1));
	vUV = uv*uvTransform.xy + uvTransform.zw;
}
