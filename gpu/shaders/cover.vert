#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

#include <common.inc>

layout(binding = 0) uniform Block {
	vec4 transform;
	vec4 uvCoverTransform;
	vec4 uvTransform;
	float z;
};

layout(location = 0) in vec2 pos;

layout(location = 0) out vec2 vCoverUV;

layout(location = 1) in vec2 uv;
layout(location = 1) out vec2 vUV;

void main() {
    gl_Position = toClipSpace(vec4(pos*transform.xy + transform.zw, z, 1));
	vUV = uv*uvTransform.xy + uvTransform.zw;
	vec3[2] fboTrans = fboTextureTransform();
	vec3 uv3 = transform3x2(fboTrans, vec3(uv, 1.0));
	vCoverUV = (uv3*vec3(uvCoverTransform.xy, 1.0)+vec3(uvCoverTransform.zw, 0.0)).xy;
}
