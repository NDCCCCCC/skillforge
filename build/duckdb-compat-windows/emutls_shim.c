// emutls shim: duckdb 预编译库用旧版 mingw emutls,本机 gcc 16.1 已移除实现。
// 引用方只有 std::__once_call / std::__once_callable 两个 thread-local 指针,
// 初始语义为 NULL,故用两个 __thread 槽模拟。
typedef struct { unsigned long size; unsigned int align; void *templ; } emutls_ctl;

/* 注意:不能加 static —— 必须是全局符号才能进入 archive 符号索引供 ld 解析 */
emutls_ctl ctl_once_call __asm__("__emutls_v._ZSt11__once_call") = {8, 8, 0};
emutls_ctl ctl_once_callable __asm__("__emutls_v._ZSt15__once_callable") = {8, 8, 0};

static __thread void *slot_once_call;
static __thread void *slot_once_callable;

void *__emutls_get_address(void *ctl) {
    if (ctl == (void *)&ctl_once_call) return &slot_once_call;
    return &slot_once_callable;
}
