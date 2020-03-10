#version 310 es
  
// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

#include <common.inc>

layout(location = 0) in vec2 pos;
layout(location = 1) in vec2 uv;

layout(binding = 0) uniform Block {
	vec4 uvTransform;
	vec4 subUVTransform;
};

layout(location = 0) out vec2 vUV;

void main() {
  vec3[2] fboTrans = fboTransform();
  vec3 p = transform3x2(fboTrans, vec3(pos, 1.0));
  gl_Position = vec4(p, 1);
  vec3[2] fboTexTrans = fboTextureTransform();
  vec3 uv3 = transform3x2(fboTexTrans, vec3(uv, 1.0));
  vUV = uv3.xy*subUVTransform.xy + subUVTransform.zw;
  vUV = transform3x2(fboTexTrans, vec3(vUV, 1.0)).xy;
  vUV = vUV*uvTransform.xy + uvTransform.zw;
}
