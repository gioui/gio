// SPDX-License-Identifier: Unlicense OR MIT

__attribute__ ((visibility ("hidden"))) jint gio_jni_GetEnv(JavaVM *vm, JNIEnv **env, jint version);
__attribute__ ((visibility ("hidden"))) jint gio_jni_AttachCurrentThread(JavaVM *vm, JNIEnv **p_env, void *thr_args);
__attribute__ ((visibility ("hidden"))) jint gio_jni_DetachCurrentThread(JavaVM *vm);

__attribute__ ((visibility ("hidden"))) jobject gio_jni_NewGlobalRef(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) void gio_jni_DeleteGlobalRef(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) jclass gio_jni_GetObjectClass(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) jmethodID gio_jni_GetStaticMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig);
__attribute__ ((visibility ("hidden"))) jmethodID gio_jni_GetMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig);
__attribute__ ((visibility ("hidden"))) jint gio_jni_CallStaticIntMethodII(JNIEnv *env, jclass clazz, jmethodID methodID, jint a1, jint a2);
__attribute__ ((visibility ("hidden"))) jfloat gio_jni_CallFloatMethod(JNIEnv *env, jobject obj, jmethodID methodID);
__attribute__ ((visibility ("hidden"))) jint gio_jni_CallIntMethod(JNIEnv *env, jobject obj, jmethodID methodID);
__attribute__ ((visibility ("hidden"))) void gio_jni_CallVoidMethod(JNIEnv *env, jobject obj, jmethodID methodID);
__attribute__ ((visibility ("hidden"))) void gio_jni_CallVoidMethod_J(JNIEnv *env, jobject obj, jmethodID methodID, jlong a1);
__attribute__ ((visibility ("hidden"))) jbyte *gio_jni_GetByteArrayElements(JNIEnv *env, jbyteArray arr);
__attribute__ ((visibility ("hidden"))) void gio_jni_ReleaseByteArrayElements(JNIEnv *env, jbyteArray arr, jbyte *bytes);
__attribute__ ((visibility ("hidden"))) jsize gio_jni_GetArrayLength(JNIEnv *env, jbyteArray arr);
__attribute__ ((visibility ("hidden"))) void gio_jni_RegisterFragment(JNIEnv *env, jobject view, jmethodID mid, char* del);
