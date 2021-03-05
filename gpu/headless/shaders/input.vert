#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

layout(location=0) in vec4 position;

void main() {
	gl_Position = position;
}
