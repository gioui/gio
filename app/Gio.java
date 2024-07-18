// SPDX-License-Identifier: Unlicense OR MIT

package org.gioui;

import android.content.ClipboardManager;
import android.content.ClipData;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;

import java.io.UnsupportedEncodingException;

public final class Gio {
	private static final Object initLock = new Object();
	private static boolean jniLoaded;
	private static final Handler handler = new Handler(Looper.getMainLooper());

	/**
	 * init loads and initializes the Go native library and runs
	 * the Go main function.
	 *
	 * It is exported for use by Android apps that need to run Go code
	 * outside the lifecycle of the Gio activity.
	 */
	public static synchronized void init(Context appCtx) {
		synchronized (initLock) {
			if (jniLoaded) {
				return;
			}
			String dataDir = appCtx.getFilesDir().getAbsolutePath();
			byte[] dataDirUTF8;
			try {
				dataDirUTF8 = dataDir.getBytes("UTF-8");
			} catch (UnsupportedEncodingException e) {
				throw new RuntimeException(e);
			}
			System.loadLibrary("gio");
			runGoMain(dataDirUTF8, appCtx);
			jniLoaded = true;
		}
	}

	static private native void runGoMain(byte[] dataDir, Context context);

	static void writeClipboard(Context ctx, String s) {
		ClipboardManager m = (ClipboardManager)ctx.getSystemService(Context.CLIPBOARD_SERVICE);
		m.setPrimaryClip(ClipData.newPlainText(null, s));
	}

	static String readClipboard(Context ctx) {
		ClipboardManager m = (ClipboardManager)ctx.getSystemService(Context.CLIPBOARD_SERVICE);
		ClipData c = m.getPrimaryClip();
		if (c == null || c.getItemCount() < 1) {
			return null;
		}
		return c.getItemAt(0).coerceToText(ctx).toString();
	}

	static void wakeupMainThread() {
		handler.post(new Runnable() {
			@Override public void run() {
				scheduleMainFuncs();
			}
		});
	}

	static private native void scheduleMainFuncs();

	static Intent startForegroundService(Context ctx, String title, String text) throws ClassNotFoundException {
		Intent intent = new Intent();
		intent.setClass(ctx, ctx.getClassLoader().loadClass("org/gioui/GioForegroundService"));
		intent.putExtra("title", title);
		intent.putExtra("text", text);

		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
			// Use startForegroundService for API level 26 and later
			ctx.startForegroundService(intent);
		} else {
			// Use startService for API level 25 and earlier
			ctx.startService(intent);
		}

		return intent;
	}
}
