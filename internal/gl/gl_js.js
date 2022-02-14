(() => {

    window._fastReadPixels = function (fn, x, y, width, height, format, ty, dataPtr, dataLen) {
        fn(x, y, width, height, format, ty, new Uint8Array(go._inst.exports.mem.buffer, dataPtr, dataLen))
    }

    window._fastBufferData = function (fn, target, dataPtr, dataLen, usage) {
        fn(target, new Uint8Array(go._inst.exports.mem.buffer, dataPtr, dataLen), usage)
    }

    window._fastBufferSubData = function (fn, target, offset, dataPtr, dataLen) {
        fn(target, offset, new Uint8Array(go._inst.exports.mem.buffer, dataPtr, dataLen))
    }

    window._fastTexSubImage2D = function (fn, target, level, x, y, width, height, format, ty, dataPtr, dataLen) {
        fn(target, level, x, y, width, height, format, ty, new Uint8Array(go._inst.exports.mem.buffer, dataPtr, dataLen))
    }
})();