// 应用版本号 —— 全项目唯一的来源。
//
// 除了「关于」对话框会显示它，打包脚本 packaging/build-deb.sh 也从这里读，
// 由它决定 deb 的 Version 和产物文件名。所以改版本只改这一处，
// 不会再出现「界面写 2.0.0.0、deb 打成 2.0.1.0」这种对不上的情况。
//
// 格式固定为四段纯数字（UOS 应用商店规范要求）：
//   MAJOR.MINOR.PATCH.BUILD
export const APP_VERSION = "2.0.0.0";
