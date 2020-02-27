#version 310 es
  
// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

layout(location = 0) in vec2 pos;
layout(location = 1) in vec2 uv;

layout(binding = 0) uniform Block {
	vec4 uvTransform;
};

layout(location = 0) out vec2 vUV;

void main() {
  vec2 p = pos;
  p.y = -p.y;
  gl_Position = vec4(p, 0, 1);
  vUV = uv*uvTransform.xy + uvTransform.zw;
}
