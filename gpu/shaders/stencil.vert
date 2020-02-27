#version 310 es

// SPDX-License-Identifier: Unlicense OR MIT

precision highp float;

layout(binding = 0) uniform Block {
	vec4 transform;
	vec2 pathOffset;
};

layout(location=0) in ivec2 corner;
layout(location=1) in float maxy;
layout(location=2) in vec2 from;
layout(location=3) in vec2 ctrl;
layout(location=4) in vec2 to;

layout(location=0) out vec2 vFrom;
layout(location=1) out vec2 vCtrl;
layout(location=2) out vec2 vTo;

void main() {
	// Add a one pixel overlap so curve quads cover their
	// entire curves. Could use conservative rasterization
	// if available.
	vec2 from = from + pathOffset;
	vec2 ctrl = ctrl + pathOffset;
	vec2 to = to + pathOffset;
	float maxy = maxy + pathOffset.y;
	vec2 pos;
	if (corner.x > 0) {
		// East.
		pos.x = max(max(from.x, ctrl.x), to.x)+1.0;
	} else {
		// West.
		pos.x = min(min(from.x, ctrl.x), to.x)-1.0;
	}
	if (corner.y > 0) {
		// North.
		pos.y = maxy + 1.0;
	} else {
		// South.
		pos.y = min(min(from.y, ctrl.y), to.y) - 1.0;
	}
	vFrom = from-pos;
	vCtrl = ctrl-pos;
	vTo = to-pos;
	pos = pos*transform.xy + transform.zw;
    gl_Position = vec4(pos, 1, 1);
}

