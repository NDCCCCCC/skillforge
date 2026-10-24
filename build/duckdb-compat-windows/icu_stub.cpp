// IcuExtension stub: 以空实现满足 libduckdb_static.a 中
// generated_extension_loader.cpp 对 duckdb::IcuExtension 的符号引用。
// 本项目不使用 ICU 扩展功能(与历史"剔除 ICU"方案一致)。
#include <string>

namespace duckdb {

class ExtensionLoader;

class IcuExtension {
public:
    static std::string Name();
    std::string Version() const;
    static void Load(ExtensionLoader &);
    virtual ~IcuExtension();
};

std::string IcuExtension::Name() { return "icu"; }
std::string IcuExtension::Version() const { return "0"; }
void IcuExtension::Load(ExtensionLoader &) {}
IcuExtension::~IcuExtension() {}

} // namespace duckdb
