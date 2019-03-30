// SPDX-License-Identifier: Unlicense OR MIT

package org.gioui;

import android.content.Context;
import android.os.Handler;
import android.util.AttributeSet;
import android.text.Editable;
import android.view.Choreographer;
import android.view.KeyCharacterMap;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.View;
import android.view.Surface;
import android.view.SurfaceView;
import android.view.SurfaceHolder;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;
import android.view.inputmethod.EditorInfo;

public class GioView extends SurfaceView implements Choreographer.FrameCallback {
	private final SurfaceHolder.Callback callbacks;
	private final InputMethodManager imm;
	private final Handler handler;
	private long nhandle;

	public GioView(Context context) {
		this(context, null);
	}

	public GioView(Context context, AttributeSet attrs) {
		super(context, attrs);
		handler = new Handler();
		imm = (InputMethodManager)context.getSystemService(Context.INPUT_METHOD_SERVICE);
		setFocusable(true);
		setFocusableInTouchMode(true);
		callbacks = new SurfaceHolder.Callback() {
			@Override public void surfaceCreated(SurfaceHolder holder) {
				// Ignore; surfaceChanged is guaranteed to be called immediately after this.
			}
			@Override public void surfaceChanged(SurfaceHolder holder, int format, int width, int height) {
				onSurfaceChanged(nhandle, getHolder().getSurface());
			}
			@Override public void surfaceDestroyed(SurfaceHolder holder) {
				onSurfaceDestroyed(nhandle);
			}
		};
		getHolder().addCallback(callbacks);
		nhandle = onCreateView(this);
	}

	@Override public boolean onKeyDown(int keyCode, KeyEvent event) {
		onKeyEvent(nhandle, keyCode, event.getUnicodeChar(), event.getEventTime());
		return false;
	}

	@Override public boolean onTouchEvent(MotionEvent event) {
		for (int j = 0; j < event.getHistorySize(); j++) {
			long time = event.getHistoricalEventTime(j);
			for (int i = 0; i < event.getPointerCount(); i++) {
				onTouchEvent(
						nhandle,
						event.ACTION_MOVE,
						event.getPointerId(i),
						event.getToolType(i),
						event.getHistoricalX(i, j),
						event.getHistoricalY(i, j),
						time);
			}
		}
		int act = event.getActionMasked();
		int idx = event.getActionIndex();
		for (int i = 0; i < event.getPointerCount(); i++) {
			int pact = event.ACTION_MOVE;
			if (i == idx) {
				pact = act;
			}
			onTouchEvent(
					nhandle,
					act,
					event.getPointerId(i),
					event.getToolType(i),
					event.getX(i),
					event.getY(i),
					event.getEventTime());
		}
		return true;
	}

	@Override public InputConnection onCreateInputConnection(EditorInfo outAttrs) {
		return new InputConnection(this);
	}

	void showTextInput() {
		post(new Runnable() {
			@Override public void run() {
				GioView.this.requestFocus();
				imm.showSoftInput(GioView.this, 0);
			}
		});
	}

	void hideTextInput() {
		post(new Runnable() {
			@Override public void run() {
				imm.hideSoftInputFromWindow(getWindowToken(), 0);
			}
		});
	}

	void postFrameCallbackOnMainThread() {
		handler.post(new Runnable() {
			@Override public void run() {
				postFrameCallback();
			}
		});
	}

	void postFrameCallback() {
		Choreographer.getInstance().removeFrameCallback(this);
		Choreographer.getInstance().postFrameCallback(this);
	}

	@Override public void doFrame(long nanos) {
		onFrameCallback(nhandle, nanos);
	}

	int getDensity() {
		return getResources().getDisplayMetrics().densityDpi;
	}

	float getFontScale() {
		return getResources().getConfiguration().fontScale;
	}

	void start() {
		onStartView(nhandle);
	}

	void stop() {
		onStopView(nhandle);
	}

	void destroy() {
		getHolder().removeCallback(callbacks);
		onDestroyView(nhandle);
		nhandle = 0;
	}

	void configurationChanged() {
		onConfigurationChanged(nhandle);
	}

	void lowMemory() {
		onLowMemory();
	}

	static private native long onCreateView(GioView view);
	static private native void onDestroyView(long handle);
	static private native void onStartView(long handle);
	static private native void onStopView(long handle);
	static private native void onSurfaceDestroyed(long handle);
	static private native void onSurfaceChanged(long handle, Surface surface);
	static private native void onConfigurationChanged(long handle);
	static private native void onLowMemory();
	static private native void onTouchEvent(long handle, int action, int pointerID, int tool, float x, float y, long time);
	static private native void onKeyEvent(long handle, int code, int character, long time);
	static private native void onFrameCallback(long handle, long nanos);

	private static class InputConnection extends BaseInputConnection {
		private final Editable editable;

		InputConnection(View view) {
			super(view, true);
			editable = Editable.Factory.getInstance().newEditable("");
		}

		@Override public Editable getEditable() {
			return editable;
		}
	}
}
