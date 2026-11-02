import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
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
import { HttpUtil } from '@/utils';

type AetherMode = 'masque' | 'gool' | 'mim';
type PsiphonMode = '' | 'chain' | 'reverse' | 'only';

interface AetherSettings {
  enabled: boolean;
  socksPort: number;
  mode: AetherMode;
  psiphon: PsiphonMode;
  psiphonRegion: string;
  psiphonCdnSets: string;
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
  psiphon: '',
  psiphonRegion: '',
  psiphonCdnSets: '',
  extraArgs: [],
};

function AetherPage() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [settings, setSettings] = useState<AetherSettings>(defaultSettings);
  const [status, setStatus] = useState<AetherStatus | null>(null);

  const load = async () => {
    const msg = await HttpUtil.get<AetherStatusView>('/panel/api/aether/status', undefined, {
      silent: true,
    });
    if (msg.success && msg.obj) {
      setSettings({ ...defaultSettings, ...msg.obj.settings });
      setStatus(msg.obj.status);
    }
    setLoading(false);
  };

  useEffect(() => {
    void load();
  }, []);

  const save = async () => {
    setSaving(true);
    const msg = await HttpUtil.post('/panel/api/aether/settings', settings);
    setSaving(false);
    if (msg.success) void load();
  };

  const restart = async () => {
    setSaving(true);
    await HttpUtil.post('/panel/api/aether/restart', {});
    setSaving(false);
    void load();
  };

  const psiphonActive = settings.psiphon !== '';

  return (
    <Layout>
      <AppSidebar />
      <Layout.Content style={{ padding: 24 }}>
        <Typography.Title level={3}>{t('pages.aether.title')}</Typography.Title>
        <Typography.Paragraph type="secondary">{t('pages.aether.subtitle')}</Typography.Paragraph>

        {loading ? (
          <Spin />
        ) : (
          <Row gutter={16}>
            <Col xs={24} lg={16}>
              <Card>
                <Form layout="vertical">
                  <Form.Item label={t('pages.aether.enable')} help={t('pages.aether.enableHelp')}>
                    <Switch
                      checked={settings.enabled}
                      onChange={(enabled) => setSettings((s) => ({ ...s, enabled }))}
                    />
                  </Form.Item>

                  <Form.Item label={t('pages.aether.socksPort')} help={t('pages.aether.socksPortHelp')}>
                    <InputNumber
                      min={0}
                      max={65535}
                      value={settings.socksPort}
                      onChange={(socksPort) =>
                        setSettings((s) => ({ ...s, socksPort: Number(socksPort) || 0 }))
                      }
                    />
                  </Form.Item>

                  <Form.Item label={t('pages.aether.mode')}>
                    <Select<AetherMode>
                      value={settings.mode}
                      onChange={(mode) => setSettings((s) => ({ ...s, mode }))}
                      options={[
                        { value: 'masque', label: t('pages.aether.modeMasque') },
                        { value: 'gool', label: t('pages.aether.modeGool') },
                        { value: 'mim', label: t('pages.aether.modeMim') },
                      ]}
                    />
                  </Form.Item>

                  <Form.Item label={t('pages.aether.psiphon')}>
                    <Select<PsiphonMode>
                      value={settings.psiphon}
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
                        label={t('pages.aether.psiphonRegion')}
                        help={t('pages.aether.psiphonRegionHelp')}
                      >
                        <Input
                          maxLength={2}
                          value={settings.psiphonRegion}
                          onChange={(e) =>
                            setSettings((s) => ({ ...s, psiphonRegion: e.target.value.toUpperCase() }))
                          }
                        />
                      </Form.Item>
                      <Form.Item
                        label={t('pages.aether.psiphonCdnSets')}
                        help={t('pages.aether.psiphonCdnSetsHelp')}
                      >
                        <Input
                          value={settings.psiphonCdnSets}
                          onChange={(e) =>
                            setSettings((s) => ({ ...s, psiphonCdnSets: e.target.value }))
                          }
                        />
                      </Form.Item>
                    </>
                  )}

                  <Form.Item label={t('pages.aether.extraArgs')} help={t('pages.aether.extraArgsHelp')}>
                    <Input.TextArea
                      rows={3}
                      value={settings.extraArgs.join('\n')}
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
                    <Button type="primary" loading={saving} onClick={() => void save()}>
                      {t('pages.aether.save')}
                    </Button>
                    <Button loading={saving} onClick={() => void restart()}>
                      {t('pages.aether.restart')}
                    </Button>
                  </Space>
                </Form>
              </Card>
            </Col>

            <Col xs={24} lg={8}>
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
            </Col>
          </Row>
        )}
      </Layout.Content>
    </Layout>
  );
}

export default AetherPage;
