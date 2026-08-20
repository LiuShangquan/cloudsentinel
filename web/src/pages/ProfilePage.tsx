import { Clock3, KeyRound, ShieldCheck, UserRound } from "lucide-react";
import { useAuth } from "../auth/AuthContext";
import { Badge, PageHeader, Panel } from "../components/ui";
import { formatDate } from "../lib/format";

export function ProfilePage() {
  const { user } = useAuth();
  return <>
    <PageHeader eyebrow="账户" title="账户信息" description="查看当前登录身份和安全会话信息。" />
    <div className="profile-grid"><Panel className="profile-card"><div className="profile-identity"><span className="profile-avatar"><UserRound size={30} /></span><div><h2>{user?.username}</h2><p>CloudSentinel 平台管理员</p><Badge status={user?.status} /></div></div><div className="profile-details"><div><span>用户 ID</span><strong>#{user?.id}</strong></div><div><span>最近登录</span><strong>{formatDate(user?.last_login_at)}</strong></div><div><span>账户创建时间</span><strong>{formatDate(user?.created_at)}</strong></div></div></Panel><Panel title="安全说明" description="当前版本的身份认证与会话边界"><div className="security-list"><div><ShieldCheck /><span><strong>JWT HS256 身份认证</strong><small>后端校验签名算法、签发者与过期时间</small></span></div><div><KeyRound /><span><strong>会话级令牌存储</strong><small>关闭浏览器标签页后，前端会话令牌自动清除</small></span></div><div><Clock3 /><span><strong>有界网络请求</strong><small>管理请求默认 15 秒超时，避免页面无限等待</small></span></div></div></Panel></div>
  </>;
}
