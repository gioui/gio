// SPDX-License-Identifier: Unlicense OR MIT

package org.gioui;

import android.content.Context;

import java.io.UnsupportedEncodingException;

public class Gio {
	private final static Object initLock = new Object();
	private static boolean jniLoaded;

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
}
