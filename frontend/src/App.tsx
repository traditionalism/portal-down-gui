import { useEffect, useMemo, useRef, useState } from "react";
import {
  Authenticate,
  DownloadAssets,
  FetchAssets,
  LoadConfig,
  PickDirectory,
  SaveConfig,
  TestWebhook,
} from "../wailsjs/go/main/App";
import { EventsOff, EventsOn } from "../wailsjs/runtime/runtime";
import { main } from "../wailsjs/go/models";
import "@/index.css";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  CheckCircle2,
  Download,
  FolderOpen,
  Loader2,
  RefreshCw,
  Search,
} from "lucide-react";

type AssetGrant = main.AssetGrant;
type Config = main.Config;

const emptyConfig: Config = {
  forumToken: "",
  discordWebhookUrl: "",
  downloadDir: "",
  useDiscord: false,
};

function formatVersion(g: AssetGrant): string {
  const v = g.asset?.versions?.[0];
  if (!v) return "—";
  return v.version || `#${v.id}`;
}

export default function App() {
  const [cfg, setCfg] = useState<Config>(emptyConfig);
  const [cfgDirty, setCfgDirty] = useState(false);
  const [authed, setAuthed] = useState(false);
  const [busy, setBusy] = useState<
    null | "auth" | "fetch" | "download" | "webhook"
  >(null);
  const [assets, setAssets] = useState<AssetGrant[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [filter, setFilter] = useState("");
  const [log, setLog] = useState<string[]>([]);
  const [progress, setProgress] = useState<{
    index: number;
    total: number;
  } | null>(null);
  const logRef = useRef<HTMLDivElement>(null);

  const updateCfg = <K extends keyof Config>(k: K, v: Config[K]) => {
    setCfg((c) => ({ ...c, [k]: v }));
    setCfgDirty(true);
  };

  useEffect(() => {
    LoadConfig()
      .then((c) => setCfg({ ...emptyConfig, ...c }))
      .catch(() => {});

    const offLog = EventsOn("log", (line: string) => {
      const stamp = new Date().toLocaleTimeString();
      setLog((l) => [...l.slice(-499), `[${stamp}] ${line}`]);
    });
    const offProg = EventsOn(
      "download:progress",
      (data: { index: number; total: number }) => {
        setProgress({ index: data.index, total: data.total });
      },
    );
    return () => {
      offLog?.();
      offProg?.();
      EventsOff("log");
      EventsOff("download:progress");
    };
  }, []);

  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [log]);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return assets;
    return assets.filter(
      (a) =>
        a.asset.name.toLowerCase().includes(q) ||
        String(a.asset.id).includes(q) ||
        String(a.grant_id).includes(q),
    );
  }, [assets, filter]);

  const allFilteredSelected =
    filtered.length > 0 && filtered.every((a) => selected.has(a.grant_id));

  const toggleAll = () => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (allFilteredSelected) filtered.forEach((a) => next.delete(a.grant_id));
      else filtered.forEach((a) => next.add(a.grant_id));
      return next;
    });
  };

  const toggleOne = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

  const saveCfg = async () => {
    await SaveConfig(cfg);
    setCfgDirty(false);
  };

  const onConnect = async () => {
    if (!cfg.forumToken.trim()) return;
    setBusy("auth");
    try {
      if (cfgDirty) await SaveConfig(cfg);
      setCfgDirty(false);
      await Authenticate(cfg.forumToken);
      setAuthed(true);
      const grants = await FetchAssets();
      setAssets(grants ?? []);
    } catch (e) {
      setAuthed(false);
      pushLog(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  };

  const onRefresh = async () => {
    setBusy("fetch");
    try {
      const grants = await FetchAssets();
      setAssets(grants ?? []);
    } finally {
      setBusy(null);
    }
  };

  const onTestWebhook = async () => {
    setBusy("webhook");
    try {
      await TestWebhook(cfg.discordWebhookUrl);
      pushLog("webhook OK");
    } catch (e: any) {
      pushLog(`webhook failed: ${e?.message ?? e}`);
    } finally {
      setBusy(null);
    }
  };

  const onPickDir = async () => {
    const dir = await PickDirectory();
    if (dir) updateCfg("downloadDir", dir);
  };

  const onDownload = async (mode: "selected" | "all") => {
    const ids =
      mode === "all" ? assets.map((a) => a.grant_id) : Array.from(selected);
    if (ids.length === 0) return;
    setBusy("download");
    setProgress({ index: 0, total: ids.length });
    try {
      await DownloadAssets(
        ids,
        cfg.useDiscord,
        cfg.downloadDir,
        cfg.discordWebhookUrl,
      );
    } finally {
      setBusy(null);
      setTimeout(() => setProgress(null), 1500);
    }
  };

  const pushLog = (msg: string) => {
    const stamp = new Date().toLocaleTimeString();
    setLog((l) => [...l.slice(-499), `[${stamp}] ${msg}`]);
  };

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="border-b border-border bg-card">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-3">
            <div>
              <h1 className="text-lg font-semibold tracking-tight">
                portal-down
              </h1>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {authed ? (
              <Badge variant="success" className="gap-1">
                <CheckCircle2 className="size-3" /> connected
              </Badge>
            ) : (
              <Badge variant="muted">disconnected</Badge>
            )}
          </div>
        </div>
      </header>

      <main className="mx-auto grid max-w-6xl gap-6 px-6 py-6 lg:grid-cols-[360px_1fr]">
        {/* Left column: settings */}
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Connection</CardTitle>
              <CardDescription>
                Paste your forum.cfx.re session cookie token (<code>_t</code>).
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="token">Forum token</Label>
                <Input
                  id="token"
                  type="password"
                  placeholder="_t cookie value"
                  value={cfg.forumToken}
                  onChange={(e) => updateCfg("forumToken", e.target.value)}
                  autoComplete="off"
                  spellCheck={false}
                />
              </div>
              <Button
                onClick={onConnect}
                disabled={busy !== null || !cfg.forumToken.trim()}
                className="w-full"
              >
                {busy === "auth" ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : authed ? (
                  <RefreshCw className="size-4" />
                ) : (
                  <Download className="size-4" />
                )}
                {authed ? "Reconnect & refresh" : "Connect"}
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Output</CardTitle>
              <CardDescription>
                Where downloaded packs are saved.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-2">
                <Label htmlFor="dir">Folder</Label>
                <div className="flex gap-2">
                  <Input
                    id="dir"
                    value={cfg.downloadDir}
                    onChange={(e) => updateCfg("downloadDir", e.target.value)}
                    spellCheck={false}
                    className="font-mono text-xs"
                  />
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={onPickDir}
                    title="Browse"
                  >
                    <FolderOpen className="size-4" />
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Discord</CardTitle>
              <CardDescription>
                Optionally mirror downloads to a webhook.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between gap-3">
                <div className="space-y-0.5">
                  <Label className="text-sm">Upload to Discord</Label>
                  <p className="text-xs text-muted-foreground">
                    Files &gt; 25 MB post a notice instead.
                  </p>
                </div>
                <Switch
                  checked={cfg.useDiscord}
                  onCheckedChange={(v) => updateCfg("useDiscord", v)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="hook">Webhook URL</Label>
                <Input
                  id="hook"
                  type="password"
                  placeholder="https://discord.com/api/webhooks/..."
                  value={cfg.discordWebhookUrl}
                  onChange={(e) =>
                    updateCfg("discordWebhookUrl", e.target.value)
                  }
                  autoComplete="off"
                  spellCheck={false}
                  disabled={!cfg.useDiscord}
                />
              </div>
              <Button
                variant="outline"
                onClick={onTestWebhook}
                disabled={busy !== null || !cfg.discordWebhookUrl}
                className="w-full"
              >
                {busy === "webhook" && (
                  <Loader2 className="size-4 animate-spin" />
                )}
                Test webhook
              </Button>
            </CardContent>
          </Card>

          <Button
            variant="secondary"
            onClick={saveCfg}
            disabled={!cfgDirty}
            className="w-full"
          >
            {cfgDirty ? "Save settings" : "Saved"}
          </Button>
        </div>

        {/* Right column: assets + log */}
        <div className="flex min-h-0 flex-col gap-6">
          <Card className="flex flex-1 flex-col">
            <CardHeader>
              <div className="flex items-start justify-between gap-4">
                <div>
                  <CardTitle>Assets</CardTitle>
                  <CardDescription>
                    {assets.length === 0
                      ? "Connect to load your asset grants."
                      : `${assets.length} grants · ${selected.size} selected`}
                  </CardDescription>
                </div>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={onRefresh}
                    disabled={!authed || busy !== null}
                  >
                    {busy === "fetch" ? (
                      <Loader2 className="size-4 animate-spin" />
                    ) : (
                      <RefreshCw className="size-4" />
                    )}
                    Refresh
                  </Button>
                  <Button
                    size="sm"
                    onClick={() => onDownload("selected")}
                    disabled={!authed || busy !== null || selected.size === 0}
                  >
                    <Download className="size-4" />
                    Selected ({selected.size})
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => onDownload("all")}
                    disabled={!authed || busy !== null || assets.length === 0}
                  >
                    Download all
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent className="flex flex-1 flex-col gap-3 min-h-0">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="filter by name or id"
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                  className="pl-9"
                />
              </div>

              <div className="rounded-md border border-border">
                <div className="flex items-center gap-3 border-b border-border bg-muted/30 px-3 py-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  <Checkbox
                    checked={allFilteredSelected}
                    onCheckedChange={toggleAll}
                    disabled={filtered.length === 0}
                  />
                  <span className="flex-1">name</span>
                  <span className="w-20 text-right">id</span>
                  <span className="w-28 text-right">version</span>
                </div>
                <div className="max-h-105 overflow-y-auto">
                  {filtered.length === 0 ? (
                    <div className="flex h-32 items-center justify-center text-sm text-muted-foreground">
                      {assets.length === 0 ? "no assets loaded" : "no matches"}
                    </div>
                  ) : (
                    filtered.map((g) => {
                      const checked = selected.has(g.grant_id);
                      return (
                        <label
                          key={g.grant_id}
                          className="flex cursor-pointer items-center gap-3 border-b border-border px-3 py-2 text-sm last:border-b-0 hover:bg-muted/40"
                        >
                          <Checkbox
                            checked={checked}
                            onCheckedChange={() => toggleOne(g.grant_id)}
                          />
                          <span className="flex-1 truncate font-medium">
                            {g.asset.name}
                          </span>
                          <span className="w-20 text-right font-mono text-xs text-muted-foreground">
                            {g.asset.id}
                          </span>
                          <span className="w-28 truncate text-right font-mono text-xs text-muted-foreground">
                            {formatVersion(g)}
                          </span>
                        </label>
                      );
                    })
                  )}
                </div>
              </div>

              {progress && (
                <div className="space-y-1.5">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>downloading…</span>
                    <span className="font-mono">
                      {progress.index}/{progress.total}
                    </span>
                  </div>
                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full bg-primary transition-all"
                      style={{
                        width:
                          progress.total > 0
                            ? `${(progress.index / progress.total) * 100}%`
                            : "0%",
                      }}
                    />
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex-row items-center justify-between space-y-0">
              <div>
                <CardTitle>Log</CardTitle>
              </div>
            </CardHeader>
            <Separator />
            <CardContent className="p-0">
              <div
                ref={logRef}
                className="h-48 overflow-y-auto bg-muted/20 p-3 font-mono text-xs leading-relaxed"
              >
                {log.length === 0 ? (
                  <div className="flex h-full items-center justify-center text-muted-foreground">
                    no activity yet
                  </div>
                ) : (
                  log.map((line, i) => (
                    <div
                      key={i}
                      className="whitespace-pre-wrap text-muted-foreground"
                    >
                      {line}
                    </div>
                  ))
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}
