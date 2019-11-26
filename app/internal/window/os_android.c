// SPDX-License-Identifier: Unlicense OR MIT

#include <jni.h>
#include <dlfcn.h>
#include <android/log.h>
#include "os_android.h"
#include "_cgo_export.h"

JNIEXPORT jint JNI_OnLoad(JavaVM *vm, void *reserved) {
	JNIEnv *env;
	if ((*vm)->GetEnv(vm, (void**)&env, JNI_VERSION_1_6) != JNI_OK) {
		return -1;
	}

	setJVM(vm);

	jclass viewClass = (*env)->FindClass(env, "org/gioui/GioView");
	if (viewClass == NULL) {
		return -1;
	}

	static const JNINativeMethod methods[] = {
		{
			.name = "runGoMain",
			.signature = "([BLandroid/content/Context;)V",
			.fnPtr = runGoMain
		},
		{
			.name = "onCreateView",
			.signature = "(Lorg/gioui/GioView;)J",
			.fnPtr = onCreateView
		},
		{
			.name = "onDestroyView",
			.signature = "(J)V",
			.fnPtr = onDestroyView
		},
		{
			.name = "onStartView",
			.signature = "(J)V",
			.fnPtr = onStartView
		},
		{
			.name = "onStopView",
			.signature = "(J)V",
			.fnPtr = onStopView
		},
		{
			.name = "onSurfaceDestroyed",
			.signature = "(J)V",
			.fnPtr = onSurfaceDestroyed
		},
		{
			.name = "onSurfaceChanged",
			.signature = "(JLandroid/view/Surface;)V",
			.fnPtr = onSurfaceChanged
		},
		{
			.name = "onConfigurationChanged",
			.signature = "(J)V",
			.fnPtr = onConfigurationChanged
		},
		{
			.name = "onWindowInsets",
			.signature = "(JIIII)V",
			.fnPtr = onWindowInsets
		},
		{
			.name = "onLowMemory",
			.signature = "()V",
			.fnPtr = onLowMemory
		},
		{
			.name = "onTouchEvent",
			.signature = "(JIIIFFIJ)V",
			.fnPtr = onTouchEvent
		},
		{
			.name = "onKeyEvent",
			.signature = "(JIIJ)V",
			.fnPtr = onKeyEvent
		},
		{
			.name = "onFrameCallback",
			.signature = "(JJ)V",
			.fnPtr = onFrameCallback
		},
		{
			.name = "onBack",
			.signature = "(J)Z",
			.fnPtr = onBack
		},
		{
			.name = "onFocusChange",
			.signature = "(JZ)V",
			.fnPtr = onFocusChange
		}
	};
	if ((*env)->RegisterNatives(env, viewClass, methods, sizeof(methods)/sizeof(methods[0])) != 0) {
		return -1;
	}

	return JNI_VERSION_1_6;
}

jint gio_jni_GetEnv(JavaVM *vm, JNIEnv **env, jint version) {
	return (*vm)->GetEnv(vm, (void **)env, version);
}

jint gio_jni_AttachCurrentThread(JavaVM *vm, JNIEnv **p_env, void *thr_args) {
	return (*vm)->AttachCurrentThread(vm, p_env, thr_args);
}

jint gio_jni_DetachCurrentThread(JavaVM *vm) {
	return (*vm)->DetachCurrentThread(vm);
}

jobject gio_jni_NewGlobalRef(JNIEnv *env, jobject obj) {
	return (*env)->NewGlobalRef(env, obj);
}

void gio_jni_DeleteGlobalRef(JNIEnv *env, jobject obj) {
	(*env)->DeleteGlobalRef(env, obj);
}

jclass gio_jni_GetObjectClass(JNIEnv *env, jobject obj) {
	return (*env)->GetObjectClass(env, obj);
}

jmethodID gio_jni_GetMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	return (*env)->GetMethodID(env, clazz, name, sig);
}

jmethodID gio_jni_GetStaticMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig) {
	return (*env)->GetStaticMethodID(env, clazz, name, sig);
}

jint gio_jni_CallStaticIntMethodII(JNIEnv *env, jclass clazz, jmethodID methodID, jint a1, jint a2) {
	return (*env)->CallStaticIntMethod(env, clazz, methodID, a1, a2);
}

jfloat gio_jni_CallFloatMethod(JNIEnv *env, jobject obj, jmethodID methodID) {
	return (*env)->CallFloatMethod(env, obj, methodID);
}

jint gio_jni_CallIntMethod(JNIEnv *env, jobject obj, jmethodID methodID) {
	return (*env)->CallIntMethod(env, obj, methodID);
}

void gio_jni_CallVoidMethod(JNIEnv *env, jobject obj, jmethodID methodID) {
	(*env)->CallVoidMethod(env, obj, methodID);
}

void gio_jni_CallVoidMethod_J(JNIEnv *env, jobject obj, jmethodID methodID, jlong a1) {
	(*env)->CallVoidMethod(env, obj, methodID, a1);
}

jbyte *gio_jni_GetByteArrayElements(JNIEnv *env, jbyteArray arr) {
	return (*env)->GetByteArrayElements(env, arr, NULL);
}

void gio_jni_ReleaseByteArrayElements(JNIEnv *env, jbyteArray arr, jbyte *bytes) {
	(*env)->ReleaseByteArrayElements(env, arr, bytes, JNI_ABORT);
}

jsize gio_jni_GetArrayLength(JNIEnv *env, jbyteArray arr) {
	return (*env)->GetArrayLength(env, arr);
}

void gio_jni_RegisterFragment(JNIEnv *env, jobject view, jmethodID mid, char* del) {
	jstring jdel = (*env)->NewStringUTF(env, del);
	(*env)->CallObjectMethod(env, view, mid, jdel);
}
