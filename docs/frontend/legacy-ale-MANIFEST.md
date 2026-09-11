# legacy-ale 冻结制品 Manifest(Phase 0,2026-09-11)

> 旧 ALE 编译产物(已做 ALE 品牌换肤外层)的冻结登记。定位:**行为/视觉 oracle + 过渡运行**,
> 不再作为源码演进对象;自主前端(`frontend/app`)按绞杀者模式逐模块替代,全部替代并
> 演练通过后本制品退出生产运行链,转为受控证据归档。
> 版权:**Do not redistribute commercially**——商业再分发走商务委托途径(用户已确认)。

## 1. 制品信息

| 项 | 值 |
|---|---|
| 运行位置 | 10.20.30.203 容器 `cabinet-rebuild-dev-frontend-1`(nginx,0.0.0.0:19173→80) |
| 制品根 | `/usr/share/nginx/html`(构建产物,无源码) |
| 框架指纹 | Vue 3 + Element Plus(打包分块 `vue-core-*` / `element-plus-*`),Vite 产物命名(`[name]-[hash].js`) |
| 外层脚本 | `ale-theme.css`(换肤批次产物)、`captcha-login.js`、`table-ux.js`、`table-fix.js`(运行时补丁,自主前端不得依赖) |
| 抓取时间 | 2026-09-11(容器运行中,`docker exec` 直读) |

## 2. 文件 SHA-256(核心产物,40 项)

```text
876766b4e80133fd490603e073d3567425b88794828a9292104244c9e40875ed  50x.html
050394313faae6f8e3eb6061bb29d722645874e059b2cc2e53db2ea1a94ef287  assets/AdminView-Br5nJM7u.css
48711abf97748d21e0e8d6b4f4e151c9c761945988cfea25d5ef296a46d813a9  assets/AdminView-CMZM9ES7.js
b89c222f1efe47e3229c2f88985211fc4ade2ee5a76b07741cfae9112e433452  assets/BatchImportDialog-B0ITbqly.css
9157b8023c26817a329fb1dafad99a83638f2152a4ec8d506dec0de245e1b1f2  assets/BatchImportDialog-BFn9_n08.js
34a5482ecb460cbe5c2efdc53b2402c40b98571bc4e69240aa55728d18e42fb6  assets/DashboardView-BKE-D84z.css
6ec3eb3c4b202354a9756d3d931337aa24986e6f8a89261554bdcaca172c8584  assets/DashboardView-DlxsGpQd.js
cddb0aa5a6dd1f8e65e398806e8d232f321036677ea2328134c48408eb0fc7b7  assets/DefaultLayout-BkCdEdm_.js
23fd911ba8ff9f6a68424d6d3a83dc7fc021e81798c155aee520823c8736329b  assets/DefaultLayout-CNFTazfD.css
5fd667ed407e37f38b07da5c865da45c6c00bd1fe3f6a969fd8fc05406c3bcc8  assets/DeviceFormFields-B18Ou5kv.js
120a5410754234b0cb062d32b7d4cec5c5123b25b28621ab4ce90b244a343395  assets/DeviceFormFields-yJayFyBm.css
917374f31550ed9cb4b213e3687eb385fb960dfa973434fb1afbb7988cd82ddc  assets/DeviceManagementView-DQc6kKpV.css
3493c330038613e9153410bea1fe9f7f837ecb9feb41286d095c4647b0ba489c  assets/DeviceManagementView-RjsQaXtb.js
9398edca817ca8f99356ead0365988fbb32b67c710b915b18f978abc0e40ec18  assets/LoginView-C_xqfAaI.css
7b7cd29a8e13559e97f22521ca46c2c74cf60bd09187a1c9c907203289c4f1e3  assets/LoginView-fq88MrWr.js
ca6cf2001d8782c69c475ebec908041a9e7a0acd359b384637b629154dd571c5  assets/RackManagementView-BL6u7R6o.css
e073eff1509eb9e8e853443d1737309bcfd1cc45d2af88016b001bf7003c5287  assets/RackManagementView-BZFxVKC6.js
bd17602fe9d00c72360af3a1875d3e9d76c90c90e5c65fc05e8a1a57ec07f0e5  assets/RackTemplateView-BsPAlSep.css
1825103f2d8d1f913d8feb1db7ef36e45abc991bfc787e02c706909ebf28a776  assets/RackTemplateView-WJGqjTF-.js
688764ef447bf7d57d8e84fac808d07a69a61e2af6d8d773ca8692c39987e8f7  assets/ResourceManagementView-Ca99QyBJ.css
d4176b8a79b17440afd64f67a6529e1a0ea8576cd5f8b8d5138e4b4fbd541bed  assets/ResourceManagementView-DR-NZxlp.js
9bcaa36a3d8963ff287ecfe8a6fcf4b57b8855c3eec4ab4343081e0c312fe4b8  assets/RoomScreenView-DTUy7kwv.js
c6f8cca7a1800b30edb1758d080553970ed3f619aad4c7795b1f8fee38fef5de  assets/RoomScreenView-DuNVEuHW.css
589d670885446566314462d71ea849c938872cde1668aee20ace23ed08ea818a  assets/ale-logo-white.png
d8f52d2efa5ec4c0aa5269563847ea7343229a6600571ec4a83db03ded09bd48  assets/ale-logo.png
de68ddbcf7c2244f205a71adb36db93ce0ed8864474947547475beab464023de  assets/ale-theme.css
2f4d6405617e34cf272d681f8fe64bd344070fc0a24e3938df6bed06202f37a5  assets/autoCode-raQXIujJ.js
aa99c2903c595dffb1cdad485caf32fb5f9c1b933b32f2a7fee671f578cb8334  assets/captcha-login.js
86f0c31e27a055537ce9972dbfc07044b81b81e2c995e9b2c3f24b46d33c7ce4  assets/device-DcGgO3DS.js
80052c08a50c2fa17b45db6f28fd7bcfa425d6b1cb4fdb2461c0cdd03fac364e  assets/element-plus-DVDI17TV.js
06a8a4972e17027dd7d4f11ef69e5921f655fb3550c3f4ac76221d8abf5c85f8  assets/element-plus-Dz4RGjfJ.css
d940aca1258d91b044056b7af6fb7aaaa08659d1ad1833970834e7403949a86e  assets/http-client-BCn5QfZZ.js
22ad909685cc7ce32293077e2952bee65dc0c0f81cec76ff8de3bad13aca4c89  assets/index-BHgR2UnN.js
cdd879714d56e32b585d85cdfc54f0cbc60d2863a9d1a2cfb758ad4e458fdd88  assets/index-C9RFk75u.css
9cdd11cee492bb1702891e35bc3b1da3864e8d4fdf28172db91e0450207c9b85  assets/rack-template-DmEgZ-eg.js
c3f0c60319341380af8f444531ee16ea26975ccf166cedc0edf8f48441f9f818  assets/rackDefaults-eUTOp6rv.js
eac7f980e417303762d136a5446d4536d7d5b1e79e4e89e83d4b4b8dd086cfd0  assets/resource-Dnoxnbws.js
d7a15147074571eab7313bcb1cb4d840d0193d14364016e62026d90690e9d193  assets/spreadsheet-7MdKTCFZ.js
c5e499a2db38abd9bc093f4aeb4f005b886e84d3626867a3c4ba79f71b0b0d8e  assets/table-fix.js
06e9a5f08781a0e4db2c1b1d721dde736a18dc38dcec22e8fa55b532f9a09fae  assets/table-ux.js
3e433ac08e990b2f6193de503c4deba77131883fd3a89562badeb20b376c2528  assets/vendor-Pl-q_uHd.js
f77427d903fec43687bb63b7379812aa49f55321202e5f0556d3c11b2a5d9abb  assets/vue-core-CrBYjdA9.js
53333e730376b7135c35afe41b4c12553bef7925e7eb90c500956d88c1d5ad83  index.html
```

(完整 40 项与容器内逐一对应;后续若容器镜像更新,须重扫并追加版本行。)

## 3. 来源与资产编号

| 项 | 值 |
|---|---|
| 上游 | 厂商机柜管理工具编译 bundle(反推工件见 `docs/反编译成果复核与下一阶段执行指南.md`) |
| 品牌化 | 2026-09-07 换肤批次(EP 变量主题,见 `ale-theme.css`;令牌基准见 `ALE-VISUAL-TOKENS.md`) |
| Logo 资产 | `F:\AIwork\Marketing Resources\ALE Brand\ALE-logos\`官方 zip(彩色/反白 PNG),实例 `ale-logo-white.png` |
| 商务资产编号 | 待确认(商务委托登记后补录) |

## 4. 退出条件(Phase 4)

全部页面自主源码替代、新旧双跑差异归零或决策登记、生产回滚演练通过后,
停止加载旧 bundle;本制品按本 manifest 归档为受控证据,不再参与生产运行。
