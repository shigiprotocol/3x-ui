import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Form,
  Input,
  InputNumber,
  Layout,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

type AetherMode = 'masque' | 'wg' | 'gool' | 'gool-classic' | 'mim';
type AetherTransport = 'h3' | 'h2';
type ScanMode = 'turbo' | 'balanced' | 'thorough' | 'verified' | 'ironclad';
type IPVersion = 'v4' | 'v6' | 'both';
type Obfuscation = '' | 'off' | 'light' | 'firewall' | 'balanced' | 'gfw' | 'aggressive';
type PsiphonMode = '' | 'chain' | 'reverse' | 'only';
type TorMode = '' | 'chain' | 'reverse' | 'only';

interface AetherSettings {
  enabled: boolean;
  socksPort: number;
  mode: AetherMode;
  transport: AetherTransport;
  scan: ScanMode;
  ipVersion: IPVersion;
  obfuscation: Obfuscation;
  fragment: boolean;
  psiphon: PsiphonMode;
  psiphonRegion: string;
  psiphonCdnSets: string;
  tor: TorMode;
  extraArgs: string[];
}

interface AetherStatus {
  running: boolean;
  ready: boolean;
  port: number;
  pid: number;
  lastErr: string;
}

interface AetherStatusView {
  settings: AetherSettings;
  status: AetherStatus;
}

const defaultSettings: AetherSettings = {
  enabled: false,
  socksPort: 2019,
  mode: 'masque',
  transport: 'h3',
  scan: 'balanced',
  ipVersion: 'v4',
  obfuscation: '',
  fragment: false,
  psiphon: '',
  psiphonRegion: '',
  psiphonCdnSets: '',
  tor: '',
  extraArgs: [],
};

// The settings payload carries an array (extraArgs); send it as JSON so the Go
// handler binds it, not as the form encoding HttpUtil defaults to.
const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

async function fetchStatus(): Promise<AetherStatusView | null> {
  const msg = await HttpUtil.get<AetherStatusView>('/panel/api/aether/status', undefined, {
    silent: true,
  });
  return msg.success && msg.obj ? msg.obj : null;
}

// The editor seeds its local state from initial once, at mount. The parent keys
// it on the loaded data so a fresh server payload remounts it with new defaults
// — no effect, so no synchronous setState-in-effect.
function AetherEditor({ initial }: { initial: AetherSettings }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [settings, setSettings] = useState<AetherSettings>(initial);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.aether.root() });

  const saveMut = useMutation({
    mutationFn: (payload: AetherSettings) =>
      HttpUtil.post('/panel/api/aether/settings', payload, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const restartMut = useMutation({
    mutationFn: () => HttpUtil.post('/panel/api/aether/restart', {}, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const busy = saveMut.isPending || restartMut.isPending;
  const psiphonActive = settings.psiphon !== '';
  const torActive = settings.tor !== '';
  const masqueBased =
    settings.mode === 'masque' || settings.mode === 'gool' || settings.mode === 'mim';

  return (
    <Form layout="vertical">
      <Form.Item
        htmlFor="aether-enabled"
        label={t('pages.aether.enable')}
        help={t('pages.aether.enableHelp')}
      >
        <Switch
          id="aether-enabled"
          checked={settings.enabled}
          onChange={(enabled) => setSettings((s) => ({ ...s, enabled }))}
        />
      </Form.Item>

      <Form.Item
        htmlFor="aether-socks-port"
        label={t('pages.aether.socksPort')}
        help={t('pages.aether.socksPortHelp')}
      >
        <InputNumber
          id="aether-socks-port"
          min={0}
          max={65535}
          value={settings.socksPort}
          onChange={(socksPort) => setSettings((s) => ({ ...s, socksPort: Number(socksPort) || 0 }))}
        />
      </Form.Item>

      <Form.Item htmlFor="aether-mode" label={t('pages.aether.mode')}>
        <Select<AetherMode>
          id="aether-mode"
          value={settings.mode}
          onChange={(mode) => setSettings((s) => ({ ...s, mode }))}
          options={[
            { value: 'masque', label: t('pages.aether.modeMasque') },
            { value: 'wg', label: t('pages.aether.modeWg') },
            { value: 'gool', label: t('pages.aether.modeGool') },
            { value: 'gool-classic', label: t('pages.aether.modeGoolClassic') },
            { value: 'mim', label: t('pages.aether.modeMim') },
          ]}
        />
      </Form.Item>

      {masqueBased && (
        <>
          <Form.Item
            htmlFor="aether-transport"
            label={t('pages.aether.transport')}
            help={t('pages.aether.transportHelp')}
          >
            <Select<AetherTransport>
              id="aether-transport"
              value={settings.transport}
              onChange={(transport) => setSettings((s) => ({ ...s, transport }))}
              options={[
                { value: 'h3', label: t('pages.aether.transportH3') },
                { value: 'h2', label: t('pages.aether.transportH2') },
              ]}
            />
          </Form.Item>

          {settings.transport === 'h2' && (
            <Form.Item
              htmlFor="aether-fragment"
              label={t('pages.aether.fragment')}
              help={t('pages.aether.fragmentHelp')}
            >
              <Switch
                id="aether-fragment"
                checked={settings.fragment}
                onChange={(fragment) => setSettings((s) => ({ ...s, fragment }))}
              />
            </Form.Item>
          )}
        </>
      )}

      <Form.Item htmlFor="aether-scan" label={t('pages.aether.scan')} help={t('pages.aether.scanHelp')}>
        <Select<ScanMode>
          id="aether-scan"
          value={settings.scan}
          onChange={(scan) => setSettings((s) => ({ ...s, scan }))}
          options={[
            { value: 'turbo', label: t('pages.aether.scanTurbo') },
            { value: 'balanced', label: t('pages.aether.scanBalanced') },
            { value: 'thorough', label: t('pages.aether.scanThorough') },
            { value: 'verified', label: t('pages.aether.scanVerified') },
            { value: 'ironclad', label: t('pages.aether.scanIronclad') },
          ]}
        />
      </Form.Item>

      <Form.Item htmlFor="aether-ip" label={t('pages.aether.ipVersion')}>
        <Select<IPVersion>
          id="aether-ip"
          value={settings.ipVersion}
          onChange={(ipVersion) => setSettings((s) => ({ ...s, ipVersion }))}
          options={[
            { value: 'v4', label: t('pages.aether.ipv4') },
            { value: 'v6', label: t('pages.aether.ipv6') },
            { value: 'both', label: t('pages.aether.ipDual') },
          ]}
        />
      </Form.Item>

      <Form.Item
        htmlFor="aether-noize"
        label={t('pages.aether.obfuscation')}
        help={t('pages.aether.obfuscationHelp')}
      >
        <Select<Obfuscation>
          id="aether-noize"
          value={settings.obfuscation}
          onChange={(obfuscation) => setSettings((s) => ({ ...s, obfuscation }))}
          options={[
            { value: '', label: t('pages.aether.obfuscationDefault') },
            { value: 'off', label: t('pages.aether.obfuscationOff') },
            { value: 'light', label: t('pages.aether.obfuscationLight') },
            { value: 'firewall', label: t('pages.aether.obfuscationFirewall') },
            { value: 'balanced', label: t('pages.aether.obfuscationBalanced') },
            { value: 'gfw', label: t('pages.aether.obfuscationGfw') },
            { value: 'aggressive', label: t('pages.aether.obfuscationAggressive') },
          ]}
        />
      </Form.Item>

      <Form.Item
        htmlFor="aether-psiphon"
        label={t('pages.aether.psiphon')}
        help={torActive ? t('pages.aether.psiphonTorExclusive') : undefined}
      >
        <Select<PsiphonMode>
          id="aether-psiphon"
          value={settings.psiphon}
          disabled={torActive}
          onChange={(psiphon) => setSettings((s) => ({ ...s, psiphon }))}
          options={[
            { value: '', label: t('pages.aether.psiphonOff') },
            { value: 'chain', label: t('pages.aether.psiphonChain') },
            { value: 'reverse', label: t('pages.aether.psiphonReverse') },
            { value: 'only', label: t('pages.aether.psiphonOnly') },
          ]}
        />
      </Form.Item>

      {psiphonActive && (
        <>
          <Form.Item
            htmlFor="aether-psiphon-region"
            label={t('pages.aether.psiphonRegion')}
            help={t('pages.aether.psiphonRegionHelp')}
          >
            <Input
              id="aether-psiphon-region"
              maxLength={2}
              value={settings.psiphonRegion}
              onChange={(e) =>
                setSettings((s) => ({ ...s, psiphonRegion: e.target.value.toUpperCase() }))
              }
            />
          </Form.Item>
          <Form.Item
            htmlFor="aether-psiphon-cdn-sets"
            label={t('pages.aether.psiphonCdnSets')}
            help={t('pages.aether.psiphonCdnSetsHelp')}
          >
            <Input
              id="aether-psiphon-cdn-sets"
              value={settings.psiphonCdnSets}
              onChange={(e) => setSettings((s) => ({ ...s, psiphonCdnSets: e.target.value }))}
            />
          </Form.Item>
        </>
      )}

      <Form.Item
        htmlFor="aether-tor"
        label={t('pages.aether.tor')}
        help={psiphonActive ? t('pages.aether.psiphonTorExclusive') : t('pages.aether.torHelp')}
      >
        <Select<TorMode>
          id="aether-tor"
          value={settings.tor}
          disabled={psiphonActive}
          onChange={(tor) => setSettings((s) => ({ ...s, tor }))}
          options={[
            { value: '', label: t('pages.aether.torOff') },
            { value: 'chain', label: t('pages.aether.torChain') },
            { value: 'reverse', label: t('pages.aether.torReverse') },
            { value: 'only', label: t('pages.aether.torOnly') },
          ]}
        />
      </Form.Item>

      <Form.Item
        htmlFor="aether-extra-args"
        label={t('pages.aether.extraArgs')}
        help={t('pages.aether.extraArgsHelp')}
      >
        <Input.TextArea
          id="aether-extra-args"
          rows={3}
          value={(settings.extraArgs ?? []).join('\n')}
          onChange={(e) =>
            setSettings((s) => ({
              ...s,
              extraArgs: e.target.value
                .split('\n')
                .map((line) => line.trim())
                .filter((line) => line.length > 0),
            }))
          }
        />
      </Form.Item>

      <Space>
        <Button
          type="primary"
          loading={saveMut.isPending}
          disabled={busy}
          onClick={() => saveMut.mutate(settings)}
        >
          {t('pages.aether.save')}
        </Button>
        <Button loading={restartMut.isPending} disabled={busy} onClick={() => restartMut.mutate()}>
          {t('pages.aether.restart')}
        </Button>
      </Space>
    </Form>
  );
}

function AetherStatusCard({ status }: { status: AetherStatus | null }) {
  const { t } = useTranslation();
  return (
    <Card title={t('pages.aether.status')}>
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Space>
          {status?.running ? (
            <Tag color="green">{t('pages.aether.statusRunning')}</Tag>
          ) : (
            <Tag>{t('pages.aether.statusStopped')}</Tag>
          )}
          {status?.running &&
            (status.ready ? (
              <Tag color="blue">{t('pages.aether.statusReady')}</Tag>
            ) : (
              <Tag color="orange">{t('pages.aether.statusNotReady')}</Tag>
            ))}
        </Space>
        <div>
          {t('pages.aether.port')}: <Typography.Text code>{status?.port ?? '-'}</Typography.Text>
        </div>
        {status?.pid ? (
          <div>
            {t('pages.aether.pid')}: <Typography.Text code>{status.pid}</Typography.Text>
          </div>
        ) : null}
        {status?.lastErr ? (
          <Typography.Text type="danger">{status.lastErr}</Typography.Text>
        ) : null}
      </Space>
    </Card>
  );
}

function AetherPage() {
  const { t } = useTranslation();
  const { antdThemeConfig } = useTheme();
  const statusQuery = useQuery({
    queryKey: keys.aether.status(),
    queryFn: fetchStatus,
    retry: false,
  });

  const loaded = statusQuery.data;
  // The backend marshals an unset extraArgs slice as JSON null, which would
  // overwrite the default []; coerce it back to an array so the editor's join()
  // never sees null.
  const initial: AetherSettings = loaded?.settings
    ? { ...defaultSettings, ...loaded.settings, extraArgs: loaded.settings.extraArgs ?? [] }
    : defaultSettings;
  // Remount the editor when a fresh server payload arrives so it reseeds without
  // an effect. updatedAt changes on every successful fetch.
  const editorKey = statusQuery.dataUpdatedAt;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <Typography.Title level={3}>{t('pages.aether.title')}</Typography.Title>
            <Typography.Paragraph type="secondary">{t('pages.aether.subtitle')}</Typography.Paragraph>

            {statusQuery.isLoading ? (
              <Spin />
            ) : (
              <Row gutter={16}>
                <Col xs={24} lg={16}>
                  <Card>
                    <AetherEditor key={editorKey} initial={initial} />
                  </Card>
                </Col>
                <Col xs={24} lg={8}>
                  <AetherStatusCard status={loaded?.status ?? null} />
                </Col>
              </Row>
            )}
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}

export default AetherPage;
