// SPDX-License-Identifier: Unlicense OR MIT

package org.gioui;
import android.app.Notification;
import android.app.Service;
import android.app.Notification;
import android.app.Notification.Builder;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.ComponentName;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.os.IBinder;
import android.os.Build;
import android.os.Bundle;

// GioForegroundService implements a Service required to use the FOREGROUND_SERVICE
// permission on Android, in order to run an application in the background.
// See https://developer.android.com/guide/components/foreground-services for
// more details. To add this permission to your application, import
// gioui.org/app/permission/foreground and use the Start method from that
// package to control this service.
public class GioForegroundService extends Service {
	private String channelName;

	// ForegroundNotificationID is a default unique ID for the tray Notification of this service, as it must be nonzero.
	private int ForegroundNotificationID = 0x42424242;

	@Override public int onStartCommand(Intent intent, int flags, int startId) {
		// Get the channel parameters from intent extras and package metadata.
		Bundle extras = intent.getExtras();
		String title = extras.getString("title");
		String text = extras.getString("text");
		Context ctx = getApplicationContext();
		try {
			ComponentName svc = new ComponentName(this, this.getClass());
			Bundle metadata = getPackageManager().getServiceInfo(svc, PackageManager.GET_META_DATA).metaData;
			if (metadata == null) {
				throw new RuntimeException("No ForegroundService MetaData found");
			}
			channelName = metadata.getString("org.gioui.ForegroundChannelName");
			String channelDesc = metadata.getString("org.gioui.ForegroundChannelDesc", "");
			String channelID = metadata.getString("org.gioui.ForegroundChannelID");
			int notificationID = metadata.getInt("org.gioui.ForegroundNotificationID", ForegroundNotificationID);
			this.createNotificationChannel(channelDesc, channelID, channelName);
			Intent launchIntent = getPackageManager().getLaunchIntentForPackage(ctx.getPackageName());

			PendingIntent pending = null;
			if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
				pending = PendingIntent.getActivity(ctx, notificationID, launchIntent, Intent.FLAG_ACTIVITY_CLEAR_TASK|PendingIntent.FLAG_IMMUTABLE);
			} else {
				pending = PendingIntent.getActivity(ctx, notificationID, launchIntent, Intent.FLAG_ACTIVITY_CLEAR_TASK);
			}
			Notification.Builder builder = new Notification.Builder(ctx, channelID)
				.setContentTitle(title)
				.setContentText(text)
				.setSmallIcon(getResources().getIdentifier("@mipmap/ic_launcher_adaptive", "drawable", getPackageName()))
				.setContentIntent(pending)
				.setPriority(Notification.PRIORITY_MIN);
			startForeground(notificationID, builder.build());
		} catch (PackageManager.NameNotFoundException e) {
			throw new RuntimeException(e);
		} catch (java.lang.SecurityException e) {
			// XXX: notify the caller of Start that the service has failed
			throw new RuntimeException(e);
		}
		return START_NOT_STICKY;
	}

	@Override public IBinder onBind(Intent intent) {
		return null;
	}

	@Override public void onCreate() {
		super.onCreate();
	}

	@Override
	public void onTaskRemoved(Intent rootIntent) {
		super.onTaskRemoved(rootIntent);
		this.deleteNotificationChannel();
		stopForeground(true);
		this.stopSelf();
	}

	@Override public void onDestroy() {
		this.deleteNotificationChannel();
	}

	private void deleteNotificationChannel() {
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
			NotificationManager notificationManager = getSystemService(NotificationManager.class);
			notificationManager.deleteNotificationChannel(channelName);
		}
	}

	private void createNotificationChannel(String channelDesc, String channelID, String channelName) {
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
			// https://developer.android.com/training/notify-user/build-notification#java
			NotificationChannel channel = new NotificationChannel(channelID, channelName, NotificationManager.IMPORTANCE_LOW);
			channel.setDescription(channelDesc);
			NotificationManager notificationManager = getSystemService(NotificationManager.class);
			notificationManager.createNotificationChannel(channel);
		}
	}
}
