// SPDX-License-Identifier: Unlicense OR MIT

package org.gioui;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.IBinder;

// GioForegroundService keeps the app's process running while it is not in
// front, showing a notification that opens the app. It is started by
// app.StartForeground, and declared by the foreground permission package.
// Its notification channel is named for the app.
public final class GioForegroundService extends Service {
	@Override public int onStartCommand(Intent intent, int flags, int startId) {
		String title = intent != null ? intent.getStringExtra("title") : null;
		String text = intent != null ? intent.getStringExtra("text") : null;
		Notification.Builder b;
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
			NotificationManager nm = getSystemService(NotificationManager.class);
			String channel = getPackageName() + ".foreground";
			CharSequence name = getApplicationInfo().loadLabel(getPackageManager());
			nm.createNotificationChannel(new NotificationChannel(channel, name, NotificationManager.IMPORTANCE_LOW));
			b = new Notification.Builder(this, channel);
		} else {
			b = new Notification.Builder(this);
		}
		Intent launch = getPackageManager().getLaunchIntentForPackage(getPackageName());
		if (launch != null) {
			b.setContentIntent(PendingIntent.getActivity(this, 0, launch, PendingIntent.FLAG_IMMUTABLE));
		}
		Notification n = b.setContentTitle(title).setContentText(text)
			.setSmallIcon(getApplicationInfo().icon != 0 ? getApplicationInfo().icon : android.R.drawable.ic_dialog_info)
			.setOngoing(true).build();
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
			startForeground(1, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE);
		} else {
			startForeground(1, n);
		}
		// Restarted alone, the process would have no Go program running in it.
		return START_NOT_STICKY;
	}

	@Override public IBinder onBind(Intent intent) {
		return null;
	}
}
