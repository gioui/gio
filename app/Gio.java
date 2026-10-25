// SPDX-License-Identifier: Unlicense OR MIT

package org.gioui;

import android.content.ClipboardManager;
import android.content.ClipData;
import android.content.ClipDescription;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.os.PersistableBundle;

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

	static void startForeground(Context ctx, String title, String text) {
		Intent i = new Intent(ctx, GioForegroundService.class);
		i.putExtra("title", title);
		i.putExtra("text", text);
		if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
			ctx.startForegroundService(i);
		} else {
			ctx.startService(i);
		}
	}

	static void stopForeground(Context ctx) {
		ctx.stopService(new Intent(ctx, GioForegroundService.class));
	}

	static void writeClipboard(Context ctx, String s, boolean sensitive) {
		ClipboardManager m = (ClipboardManager)ctx.getSystemService(Context.CLIPBOARD_SERVICE);
		ClipData clip = ClipData.newPlainText(null, s);
		// Android 13 and later hide sensitive content in the clipboard
		// preview, earlier versions ignore the extra. The constant is
		// inlined at compile time, the extras need API 24.
		if (sensitive && Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
			PersistableBundle extras = new PersistableBundle();
			extras.putBoolean(ClipDescription.EXTRA_IS_SENSITIVE, true);
			clip.getDescription().setExtras(extras);
		}
		m.setPrimaryClip(clip);
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
}
