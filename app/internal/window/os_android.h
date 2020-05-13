// SPDX-License-Identifier: Unlicense OR MIT

__attribute__ ((visibility ("hidden"))) jint gio_jni_GetEnv(JavaVM *vm, JNIEnv **env, jint version);
__attribute__ ((visibility ("hidden"))) jint gio_jni_GetJavaVM(JNIEnv *env, JavaVM **jvm);
__attribute__ ((visibility ("hidden"))) jint gio_jni_AttachCurrentThread(JavaVM *vm, JNIEnv **p_env, void *thr_args);
__attribute__ ((visibility ("hidden"))) jint gio_jni_DetachCurrentThread(JavaVM *vm);

__attribute__ ((visibility ("hidden"))) jobject gio_jni_NewGlobalRef(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) void gio_jni_DeleteGlobalRef(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) jclass gio_jni_GetObjectClass(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) jmethodID gio_jni_GetStaticMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig);
__attribute__ ((visibility ("hidden"))) jmethodID gio_jni_GetMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig);
__attribute__ ((visibility ("hidden"))) jfloat gio_jni_CallFloatMethod(JNIEnv *env, jobject obj, jmethodID methodID);
__attribute__ ((visibility ("hidden"))) jint gio_jni_CallIntMethod(JNIEnv *env, jobject obj, jmethodID methodID);
__attribute__ ((visibility ("hidden"))) void gio_jni_CallStaticVoidMethodA(JNIEnv *env, jclass cls, jmethodID methodID, const jvalue *args);
__attribute__ ((visibility ("hidden"))) void gio_jni_CallVoidMethodA(JNIEnv *env, jobject obj, jmethodID methodID, const jvalue *args);
__attribute__ ((visibility ("hidden"))) jbyte *gio_jni_GetByteArrayElements(JNIEnv *env, jbyteArray arr);
__attribute__ ((visibility ("hidden"))) void gio_jni_ReleaseByteArrayElements(JNIEnv *env, jbyteArray arr, jbyte *bytes);
__attribute__ ((visibility ("hidden"))) jsize gio_jni_GetArrayLength(JNIEnv *env, jbyteArray arr);
__attribute__ ((visibility ("hidden"))) jstring gio_jni_NewString(JNIEnv *env, const jchar *unicodeChars, jsize len);
__attribute__ ((visibility ("hidden"))) jsize gio_jni_GetStringLength(JNIEnv *env, jstring str);
__attribute__ ((visibility ("hidden"))) const jchar *gio_jni_GetStringChars(JNIEnv *env, jstring str);
__attribute__ ((visibility ("hidden"))) jthrowable gio_jni_ExceptionOccurred(JNIEnv *env);
__attribute__ ((visibility ("hidden"))) void gio_jni_ExceptionClear(JNIEnv *env);
__attribute__ ((visibility ("hidden"))) jobject gio_jni_CallObjectMethodA(JNIEnv *env, jobject obj, jmethodID method, jvalue *args);
__attribute__ ((visibility ("hidden"))) jobject gio_jni_CallStaticObjectMethodA(JNIEnv *env, jclass cls, jmethodID method, jvalue *args);
