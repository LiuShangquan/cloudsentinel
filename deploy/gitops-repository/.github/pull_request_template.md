## 变更内容

- 目标环境：
- 源代码提交：
- API / Worker / Migration digest：

## 风险与验证

- [ ] Staging 已同步且健康
- [ ] Migration 为向前兼容变更，或已附停机/恢复方案
- [ ] 不包含明文 Secret、StatefulSet 或浮动镜像标签
- [ ] 已检查 API、Worker、Incident 和 Probe 指标
- [ ] Production 回滚目标 digest 已记录
