"""T01 自测：用例中途故意失败，证明 teardown 仍彻底清理（复评 P1-N1 验收）。

- test_boom...:标记 xfail(strict=True)——call 阶段必须失败；若 teardown 清理
  失效，finalizer 的零残留断言会把它升级为 error（整体非零退出）。
- test_teardown_survived_failure:在 boom 之后运行，独立复核零残留。

预期整份文件执行结果：1 xfailed + 1 passed，退出码 0。
"""
import uuid

import pytest

from conftest import api, data_of

BOOM = {}


@pytest.mark.xfail(reason="故意失败：验证 fixture 在用例异常时仍清理资源", strict=True)
def test_boom_creates_resources_then_fails(env):
    dev1 = env["make_device"]()
    dev2 = env["make_device"]()
    BOOM["run_id"] = env["run_id"]
    BOOM["dc"] = env["dc"]
    BOOM["device_ids"] = [dev1["id"], dev2["id"]]
    raise RuntimeError(f"intentional failure after creating devices {dev1['id']}, {dev2['id']}")


def test_teardown_survived_failure(admin_token):
    assert "run_id" in BOOM, "boom 用例未执行（顺序依赖），自测无效"
    # 设备零残留
    st, devs = api("GET", "/api/v1/devices?page=1&pageSize=500", None, admin_token)
    assert st == 200, f"list devices failed: {st}"
    hit = [m.get("id") for m in (data_of(devs).get("items") or [])
           if m.get("id") in BOOM["device_ids"]]
    assert not hit, f"boom 用例失败后设备仍残留: {hit}"
    # dc 零残留（树中不存在）
    st, tree = api("GET", "/api/v1/resource-tree", None, admin_token)
    assert st == 200, f"resource-tree failed: {st}"

    def walk(node):
        if isinstance(node, dict):
            if node.get("id") == BOOM["dc"]:
                return True
            return any(walk(v) for v in node.values())
        if isinstance(node, list):
            return any(walk(it) for it in node)
        return False

    assert not walk(data_of(tree)), "boom 用例失败后 dc 仍残留在资源树"
