#!/usr/bin/env python3
"""差分回放探针：把「两套系统能力差异」写成机器可比的 INFO 用例。

在正式套件之前运行。这些用例不参与通过率，但进入 results.jsonl，
使差异在差分报告中以数据形式呈现，而不是被兼容层悄悄抹平。
"""
from gt_core import info_call, call, T


def run(admin_password):
    """记录原生行为探测结果（expect=None → INFO，不参与通过率）。"""

    # D1: 登录是否强制验证码（重建版要求，厂商原版不要求）
    info_call("DIFF-LOGIN-NO-CAPTCHA", "DIFF", "POST", "/api/v1/auth/login",
              body={"username": "admin", "password": admin_password},
              expect=None)

    # D2: 未认证访问受保护资源（两版均应 401 UNAUTHORIZED）
    info_call("DIFF-NOAUTH-RESOURCE-TREE", "DIFF", "GET", "/api/v1/resource-tree",
              expect=None)

    # D3: 响应信封形状（两版应一致：code/message/data/requestId）
    info_call("DIFF-ENVELOPE-SHAPE", "DIFF", "GET", "/api/v1/device-types",
              token=T(), expect=None)

    # D4: 未知路由的 404 语义
    info_call("DIFF-UNKNOWN-ROUTE", "DIFF", "GET", "/api/v1/__no_such_route__",
              token=T(), expect=None)
