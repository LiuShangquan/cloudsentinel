import { Activity, ArrowRight, CheckCircle2, LockKeyhole, Radar, ShieldCheck } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Button, FormField } from "../components/ui";

export function LoginPage() {
  const { user, login } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  if (user) return <Navigate to="/" replace />;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError("");
    if (!username.trim() || !password) return setError("请输入用户名和密码");
    setBusy(true);
    try {
      await login(username.trim(), password);
      navigate((location.state as { from?: string } | null)?.from ?? "/", { replace: true });
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "登录失败");
    } finally { setBusy(false); }
  };

  return <div className="login-page">
    <section className="login-story">
      <div className="login-story__brand"><div className="brand__mark"><ShieldCheck size={25} /></div><strong>CloudSentinel</strong></div>
      <div className="login-story__content"><span className="eyebrow eyebrow--light">服务可靠性控制台</span><h1>让每一次探测<br />都成为可靠性证据</h1><p>统一管理主机资产、HTTP/TCP 探测、执行结果和故障事件，让服务状态清晰、可追踪、可处置。</p><div className="login-features"><div><Radar size={20} /><span><strong>持续探测</strong><small>有界调度与可靠执行</small></span></div><div><Activity size={20} /><span><strong>实时洞察</strong><small>运行指标与结果追踪</small></span></div><div><CheckCircle2 size={20} /><span><strong>事件闭环</strong><small>确认、处理、恢复、关闭</small></span></div></div></div>
      <div className="login-story__footer">CloudSentinel Enterprise GitOps Baseline</div>
    </section>
    <section className="login-form-area"><form className="login-card" onSubmit={submit}><div className="login-card__icon"><LockKeyhole size={23} /></div><h2>登录管理控制台</h2><p>使用 CloudSentinel 平台账户继续</p><div className="login-card__fields"><FormField label="用户名"><input autoFocus autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="请输入用户名" /></FormField><FormField label="密码"><input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="请输入密码" /></FormField>{error && <div className="form-alert">{error}</div>}<Button type="submit" disabled={busy}>{busy ? "正在验证…" : <>安全登录 <ArrowRight size={17} /></>}</Button></div><div className="login-card__note">JWT 安全认证 · 会话结束后凭据自动清除</div></form></section>
  </div>;
}
