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
  Layout,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';

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

interface AetherProfile {
  id: string;
  name: string;
  settings: AetherSettings;
}

interface ProfileStatus {
  id: string;
  name: string;
  running: boolean;
  ready: boolean;
  port: number;
  dialPort: number;
  pid: number;
  lastErr: string;
}

interface AetherView {
  profiles: AetherProfile[];
  statuses: ProfileStatus[];
}

const defaultSettings: AetherSettings = {
  enabled: true,
  socksPort: 0,
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

// The settings payload carries arrays, so send JSON (HttpUtil defaults to form).
const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

function newProfileId(): string {
  try {
    return crypto.randomUUID().replace(/-/g, '').slice(0, 8);
  } catch {
    return Math.random().toString(36).slice(2, 10);
  }
}

function normalizeProfile(p: AetherProfile): AetherProfile {
  return {
    id: p.id || newProfileId(),
    name: p.name || 'Aether',
    settings: { ...defaultSettings, ...p.settings, extraArgs: p.settings?.extraArgs ?? [] },
  };
}

async function fetchView(): Promise<AetherView> {
  const msg = await HttpUtil.get<AetherView>('/panel/api/aether/status', undefined, {
    silent: true,
  });
  if (msg.success && msg.obj) {
    return {
      profiles: (msg.obj.profiles ?? []).map(normalizeProfile),
      statuses: msg.obj.statuses ?? [],
    };
  }
  return { profiles: [], statuses: [] };
}

function ProfileFields({
  value,
  onChange,
}: {
  value: AetherSettings;
  onChange: (next: AetherSettings) => void;
}) {
  const { t } = useTranslation();
  const set = (patch: Partial<AetherSettings>) => onChange({ ...value, ...patch });

  const psiphonActive = value.psiphon !== '';
  const torActive = value.tor !== '';
  const masqueBased = value.mode === 'masque' || value.mode === 'gool' || value.mode === 'mim';

  return (
    <>
      <Form.Item label={t('pages.aether.mode')}>
        <Select<AetherMode>
          aria-label={t('pages.aether.mode')}
          value={value.mode}
          onChange={(mode) => set({ mode })}
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
          <Form.Item label={t('pages.aether.transport')} help={t('pages.aether.transportHelp')}>
            <Select<AetherTransport>
              aria-label={t('pages.aether.transport')}
              value={value.transport}
              onChange={(transport) => set({ transport })}
              options={[
                { value: 'h3', label: t('pages.aether.transportH3') },
                { value: 'h2', label: t('pages.aether.transportH2') },
              ]}
            />
          </Form.Item>

          {value.transport === 'h2' && (
            <Form.Item label={t('pages.aether.fragment')} help={t('pages.aether.fragmentHelp')}>
              <Switch
                aria-label={t('pages.aether.fragment')}
                checked={value.fragment}
                onChange={(fragment) => set({ fragment })}
              />
            </Form.Item>
          )}
        </>
      )}

      <Form.Item label={t('pages.aether.scan')} help={t('pages.aether.scanHelp')}>
        <Select<ScanMode>
          aria-label={t('pages.aether.scan')}
          value={value.scan}
          onChange={(scan) => set({ scan })}
          options={[
            { value: 'turbo', label: t('pages.aether.scanTurbo') },
            { value: 'balanced', label: t('pages.aether.scanBalanced') },
            { value: 'thorough', label: t('pages.aether.scanThorough') },
            { value: 'verified', label: t('pages.aether.scanVerified') },
            { value: 'ironclad', label: t('pages.aether.scanIronclad') },
          ]}
        />
      </Form.Item>

      <Form.Item label={t('pages.aether.ipVersion')}>
        <Select<IPVersion>
          aria-label={t('pages.aether.ipVersion')}
          value={value.ipVersion}
          onChange={(ipVersion) => set({ ipVersion })}
          options={[
            { value: 'v4', label: t('pages.aether.ipv4') },
            { value: 'v6', label: t('pages.aether.ipv6') },
            { value: 'both', label: t('pages.aether.ipDual') },
          ]}
        />
      </Form.Item>

      <Form.Item label={t('pages.aether.obfuscation')} help={t('pages.aether.obfuscationHelp')}>
        <Select<Obfuscation>
          aria-label={t('pages.aether.obfuscation')}
          value={value.obfuscation}
          onChange={(obfuscation) => set({ obfuscation })}
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
        label={t('pages.aether.psiphon')}
        help={torActive ? t('pages.aether.psiphonTorExclusive') : undefined}
      >
        <Select<PsiphonMode>
          aria-label={t('pages.aether.psiphon')}
          value={value.psiphon}
          disabled={torActive}
          onChange={(psiphon) => set({ psiphon })}
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
            label={t('pages.aether.psiphonRegion')}
            help={t('pages.aether.psiphonRegionHelp')}
          >
            <Input
              aria-label={t('pages.aether.psiphonRegion')}
              maxLength={2}
              value={value.psiphonRegion}
              onChange={(e) => set({ psiphonRegion: e.target.value.toUpperCase() })}
            />
          </Form.Item>
          <Form.Item
            label={t('pages.aether.psiphonCdnSets')}
            help={t('pages.aether.psiphonCdnSetsHelp')}
          >
            <Input
              aria-label={t('pages.aether.psiphonCdnSets')}
              value={value.psiphonCdnSets}
              onChange={(e) => set({ psiphonCdnSets: e.target.value })}
            />
          </Form.Item>
        </>
      )}

      <Form.Item
        label={t('pages.aether.tor')}
        help={psiphonActive ? t('pages.aether.psiphonTorExclusive') : t('pages.aether.torHelp')}
      >
        <Select<TorMode>
          aria-label={t('pages.aether.tor')}
          value={value.tor}
          disabled={psiphonActive}
          onChange={(tor) => set({ tor })}
          options={[
            { value: '', label: t('pages.aether.torOff') },
            { value: 'chain', label: t('pages.aether.torChain') },
            { value: 'reverse', label: t('pages.aether.torReverse') },
            { value: 'only', label: t('pages.aether.torOnly') },
          ]}
        />
      </Form.Item>

      <Form.Item label={t('pages.aether.extraArgs')} help={t('pages.aether.extraArgsHelp')}>
        <Input.TextArea
          aria-label={t('pages.aether.extraArgs')}
          rows={2}
          value={(value.extraArgs ?? []).join('\n')}
          onChange={(e) =>
            set({
              extraArgs: e.target.value
                .split('\n')
                .map((line) => line.trim())
                .filter((line) => line.length > 0),
            })
          }
        />
      </Form.Item>
    </>
  );
}

function StatusBadge({ status }: { status: ProfileStatus | undefined }) {
  const { t } = useTranslation();
  if (!status || !status.running) {
    return <Tag>{t('pages.aether.statusStopped')}</Tag>;
  }
  return (
    <Space size="small">
      <Tag color="green">{t('pages.aether.statusRunning')}</Tag>
      {status.ready ? (
        <Tag color="blue">{t('pages.aether.statusReady')}</Tag>
      ) : (
        <Tag color="orange">{t('pages.aether.statusNotReady')}</Tag>
      )}
      {status.lastErr ? <Typography.Text type="danger">{status.lastErr}</Typography.Text> : null}
    </Space>
  );
}

function AetherManager({
  initial,
  statuses,
}: {
  initial: AetherProfile[];
  statuses: ProfileStatus[];
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [profiles, setProfiles] = useState<AetherProfile[]>(initial);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.aether.root() });

  const saveMut = useMutation({
    mutationFn: (payload: AetherProfile[]) =>
      HttpUtil.post('/panel/api/aether/settings', { profiles: payload }, JSON_HEADERS),
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

  const statusById = new Map(statuses.map((s) => [s.id, s]));
  const update = (id: string, patch: Partial<AetherProfile>) =>
    setProfiles((list) => list.map((p) => (p.id === id ? { ...p, ...patch } : p)));
  const remove = (id: string) => setProfiles((list) => list.filter((p) => p.id !== id));
  const add = () =>
    setProfiles((list) => [
      ...list,
      { id: newProfileId(), name: `Aether ${list.length + 1}`, settings: { ...defaultSettings } },
    ]);

  return (
    <>
      <Space style={{ marginBottom: 16 }}>
        <Button
          type="primary"
          loading={saveMut.isPending}
          disabled={busy}
          onClick={() => saveMut.mutate(profiles)}
        >
          {t('pages.aether.save')}
        </Button>
        <Button loading={restartMut.isPending} disabled={busy} onClick={() => restartMut.mutate()}>
          {t('pages.aether.restart')}
        </Button>
        <Button icon={<PlusOutlined aria-hidden />} onClick={add} disabled={busy}>
          {t('pages.aether.addProfile')}
        </Button>
      </Space>

      {profiles.length === 0 ? (
        <Typography.Text type="secondary">{t('pages.aether.noProfiles')}</Typography.Text>
      ) : (
        <Row gutter={[16, 16]}>
          {profiles.map((p) => (
            <Col key={p.id} xs={24} lg={12}>
              <Card
                title={
                  <Space>
                    <Switch
                      aria-label={t('pages.aether.enable')}
                      checked={p.settings.enabled}
                      onChange={(enabled) => update(p.id, { settings: { ...p.settings, enabled } })}
                    />
                    <Input
                      aria-label={t('pages.aether.profileName')}
                      value={p.name}
                      onChange={(e) => update(p.id, { name: e.target.value })}
                      style={{ width: 180 }}
                      placeholder={t('pages.aether.profileName')}
                    />
                  </Space>
                }
                extra={
                  <Button
                    danger
                    size="small"
                    icon={<DeleteOutlined aria-hidden />}
                    onClick={() => remove(p.id)}
                    aria-label={t('remove')}
                  />
                }
              >
                <div style={{ marginBottom: 12 }}>
                  <StatusBadge status={statusById.get(p.id)} />
                </div>
                <Form layout="vertical">
                  <ProfileFields
                    value={p.settings}
                    onChange={(settings) => update(p.id, { settings })}
                  />
                </Form>
              </Card>
            </Col>
          ))}
        </Row>
      )}
    </>
  );
}

function AetherPage() {
  const { t } = useTranslation();
  const { antdThemeConfig } = useTheme();
  const query = useQuery({ queryKey: keys.aether.status(), queryFn: fetchView, retry: false });

  const data = query.data;
  const editorKey = query.dataUpdatedAt;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <Typography.Title level={3}>{t('pages.aether.title')}</Typography.Title>
            <Typography.Paragraph type="secondary">
              {t('pages.aether.subtitle')}
            </Typography.Paragraph>

            {query.isLoading ? (
              <Spin />
            ) : (
              <AetherManager
                key={editorKey}
                initial={data?.profiles ?? []}
                statuses={data?.statuses ?? []}
              />
            )}
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}

export default AetherPage;
