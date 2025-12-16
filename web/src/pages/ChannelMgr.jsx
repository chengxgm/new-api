import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {API, showError, showInfo, showSuccess} from '../helpers';
import {Button, Input, Modal, Space, Table, TextArea, Typography, Select} from '@douyinfe/semi-ui';

const normalizeStr = (v) => {
    if (v === null || v === undefined) return '';
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
};

const ChannelMgr = () => {
    const {t} = useTranslation();

    const [metaLoading, setMetaLoading] = useState(false);
    const [loading, setLoading] = useState(false);

    const [columns, setColumns] = useState([]); // [{name,type,notnull,...}]
    const [rows, setRows] = useState([]);
    const [total, setTotal] = useState(0);

    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(50);

    const [searchKey, setSearchKey] = useState('');
    const [searchTag, setSearchTag] = useState('');
    const [searchStatus, setSearchStatus] = useState(''); // '' means all
    const [searchType, setSearchType] = useState(''); // '' means all

    const [selectedRowKeys, setSelectedRowKeys] = useState([]);
    const [selectedRows, setSelectedRows] = useState([]);

    // Edit/Create modal
    const [editVisible, setEditVisible] = useState(false);
    const [dialogMode, setDialogMode] = useState('edit'); // 'edit' | 'create'
    const [editingRow, setEditingRow] = useState(null);
    const [editForm, setEditForm] = useState({});

    // Batch update modal
    const [batchVisible, setBatchVisible] = useState(false);
    const [batchForm, setBatchForm] = useState({});

    // Batch copy modal
    const [batchCopyVisible, setBatchCopyVisible] = useState(false);
    const [batchCopyKeysText, setBatchCopyKeysText] = useState('');

    const editableColumnNames = useMemo(
        () => (columns || []).map((c) => c.name).filter((name) => name !== 'id'),
        [columns],
    );

    const channelTypeOptions = useMemo(
        () => [
            {value: '0', label: t('Unknown (0)')},
            {value: '1', label: t('OpenAI (1)')},
            {value: '3', label: t('Azure (3)')},
            {value: '14', label: t('Anthropic (14)')},
            {value: '20', label: t('OpenRouter (20)')},
            {value: '24', label: t('Gemini (24)')},
            {value: '33', label: t('Aws (33)')},
            {value: '41', label: t('VertexAi (41)')},
        ],
        [t],
    );

    const buildEmptyForm = useCallback(() => {
        const obj = {};
        editableColumnNames.forEach((name) => {
            obj[name] = '';
        });
        return obj;
    }, [editableColumnNames]);

    const buildEditFormFromRow = useCallback(
        (row) => {
            const obj = {};
            editableColumnNames.forEach((name) => {
                obj[name] = normalizeStr(row?.[name]);
            });
            return obj;
        },
        [editableColumnNames],
    );

    const loadMeta = useCallback(async () => {
        setMetaLoading(true);
        try {
            const res = await API.get('/api/channel/mgr/meta');
            setColumns(res?.data?.columns || []);
        } catch (e) {
            showError(e?.message || t('加载列信息失败'));
            setColumns([]);
        } finally {
            setMetaLoading(false);
        }
    }, [t]);

    const loadData = useCallback(
        async (targetPage = page, targetPageSize = pageSize) => {
            setLoading(true);
            try {
                const params = {
                    offset: (targetPage - 1) * targetPageSize,
                    limit: targetPageSize,
                };
                const k = (searchKey || '').trim();
                const tag = (searchTag || '').trim();
                const status = (searchStatus || '').trim();
                const type = (searchType || '').trim();
                if (k) params.key = k;
                if (tag) params.tag = tag;
                if (status !== '') params.status = status;
                if (type !== '') params.type = type;

                const res = await API.get('/api/channel/mgr/list', {params});
                setRows(res?.data?.rows || []);
                setTotal(res?.data?.total || 0);
                setPage(targetPage);
                setPageSize(targetPageSize);
                setSelectedRowKeys([]);
                setSelectedRows([]);
            } catch (e) {
                showError(e?.message || t('加载数据失败'));
            } finally {
                setLoading(false);
            }
        },
        [page, pageSize, searchKey, searchTag, searchStatus, searchType, t],
    );

    useEffect(() => {
        (async () => {
            await loadMeta();
            await loadData(1, pageSize);
        })();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const handleSearch = async () => {
        await loadData(1, pageSize);
    };

    const handleReset = async () => {
        setSearchKey('');
        setSearchTag('');
        setSearchStatus('');
        setSearchType('');
        await loadData(1, pageSize);
    };

    const openCreateDialog = () => {
        setDialogMode('create');
        setEditingRow(null);
        setEditForm(buildEmptyForm());
        setEditVisible(true);
    };

    const openEditDialog = (row) => {
        setDialogMode('edit');
        setEditingRow(row);
        setEditForm(buildEditFormFromRow(row));
        setEditVisible(true);
    };

    const submitEdit = async () => {
        try {
            if (dialogMode === 'create') {
                await API.post('/api/channel/mgr/create', {values: editForm});
                showSuccess(t('创建成功'));
                setEditVisible(false);
                await loadData(1, pageSize);
                return;
            }

            if (!editingRow?.id) {
                showError(t('缺少ID'));
                return;
            }

            const values = {};
            editableColumnNames.forEach((name) => {
                const oldVal = normalizeStr(editingRow?.[name]);
                const newVal = normalizeStr(editForm?.[name]);
                if (oldVal !== newVal) {
                    values[name] = newVal;
                }
            });

            if (Object.keys(values).length === 0) {
                showInfo(t('没有任何修改'));
                setEditVisible(false);
                return;
            }

            await API.post('/api/channel/mgr/update', {id: editingRow.id, values});
            showSuccess(t('保存成功'));
            setEditVisible(false);
            await loadData(page, pageSize);
        } catch (e) {
            showError(e?.response?.data?.error || e?.message || t('保存失败'));
        }
    };

    const handleCopyRow = async (row) => {
        try {
            await API.post('/api/channel/mgr/copy', {id: row.id});
            showSuccess(t('复制成功'));
            await loadData(page, pageSize);
        } catch (e) {
            showError(e?.response?.data?.error || e?.message || t('复制失败'));
        }
    };

    const handleDeleteRow = (row) => {
        Modal.confirm({
            title: t('确定要删除吗？'),
            content: t('该接口会在 key 非空时删除所有相同 key 的记录，此操作不可撤销。'),
            okType: 'danger',
            centered: true,
            onOk: async () => {
                try {
                    await API.post('/api/channel/mgr/delete', {id: row.id});
                    showSuccess(t('删除成功'));
                    await loadData(page, pageSize);
                } catch (e) {
                    showError(e?.response?.data?.error || e?.message || t('删除失败'));
                }
            },
        });
    };

    const openBatchEditDialog = () => {
        if (selectedRows.length === 0) {
            showInfo(t('请先选择要批量修改的行'));
            return;
        }
        setBatchForm(buildEmptyForm());
        setBatchVisible(true);
    };

    const submitBatchEdit = async () => {
        const ids = (selectedRows || []).map((r) => r.id).filter(Boolean);
        const values = {};
        Object.keys(batchForm || {}).forEach((k) => {
            const v = batchForm[k];
            if (v !== '' && v !== null && v !== undefined) {
                values[k] = v;
            }
        });

        if (ids.length === 0) {
            showInfo(t('未选中任何行'));
            return;
        }
        if (Object.keys(values).length === 0) {
            showInfo(t('没有需要更新的列'));
            return;
        }

        try {
            await API.post('/api/channel/mgr/batch_update', {ids, values});
            showSuccess(t('批量更新成功'));
            setBatchVisible(false);
            await loadData(page, pageSize);
        } catch (e) {
            showError(e?.response?.data?.error || e?.message || t('批量更新失败'));
        }
    };

    const openBatchCopyDialog = () => {
        if (selectedRows.length !== 1) {
            showInfo(t('请先选择一行作为模板'));
            return;
        }
        setBatchCopyKeysText('');
        setBatchCopyVisible(true);
    };

    const submitBatchCopy = async () => {
        if (selectedRows.length !== 1) {
            showInfo(t('请先选择一行作为模板'));
            return;
        }
        const raw = batchCopyKeysText || '';
        const keys = raw
            .split('\n')
            .map((v) => v.trim())
            .filter(Boolean);
        if (keys.length === 0) {
            showInfo(t('请至少填写一个 key'));
            return;
        }

        try {
            const templateId = selectedRows[0].id;
            const res = await API.post('/api/channel/mgr/batch_copy', {
                id: templateId,
                keys,
            });
            const count = res?.data?.count ?? keys.length;
            showSuccess(t('批量复制新增成功，共新增 {{count}} 行', {count}));
            setBatchCopyVisible(false);
            await loadData(page, pageSize);
        } catch (e) {
            showError(e?.response?.data?.error || e?.message || t('批量复制新增失败'));
        }
    };

    const tableColumns = useMemo(() => {
        const cols = (columns || []).map((col) => ({
            title: col.name,
            dataIndex: col.name,
            key: col.name,
            width: 180,
            ellipsis: true,
            render: (_, record) => normalizeStr(record?.[col.name]),
        }));

        cols.push({
            title: t('操作'),
            key: '__actions',
            fixed: 'right',
            width: 220,
            render: (_, record) => (
                <Space>
                    <Button size='small' onClick={() => openEditDialog(record)}>
                        {t('编辑')}
                    </Button>
                    <Button size='small' type='tertiary' onClick={() => handleCopyRow(record)}>
                        {t('复制')}
                    </Button>
                    <Button size='small' type='danger' onClick={() => handleDeleteRow(record)}>
                        {t('删除')}
                    </Button>
                </Space>
            ),
        });
        return cols;
    }, [columns, t]);

    const renderFormGrid = (form, setForm, hintText) => (
        <div className='flex flex-col gap-3'>
            {hintText ? (
                <Typography.Text type='tertiary'>{hintText}</Typography.Text>
            ) : null}
            <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
                {editableColumnNames.map((name) => {
                    const colMeta = columns.find((c) => c.name === name);
                    const label = colMeta?.type ? `${name} (${colMeta.type})` : name;
                    return (
                        <div key={name} className='flex flex-col gap-1'>
                            <Typography.Text strong>{label}</Typography.Text>
                            <TextArea
                                value={form?.[name] ?? ''}
                                autosize={{minRows: 1, maxRows: 3}}
                                onChange={(v) => setForm((prev) => ({...(prev || {}), [name]: v}))}
                                className='font-mono'
                            />
                        </div>
                    );
                })}
            </div>
        </div>
    );

    return (
        <div className='mt-[60px] px-2'>
            <div className='flex flex-col lg:flex-row lg:items-center justify-between gap-2'>
                <div className='flex flex-col sm:flex-row gap-2 flex-1'>
                    <Input
                        value={searchKey}
                        onChange={setSearchKey}
                        showClear
                        placeholder={t('按 key 查询')}
                    />
                    <Input
                        value={searchTag}
                        onChange={setSearchTag}
                        showClear
                        placeholder={t('按 tag 查询')}
                    />
                    <Select
                        value={searchStatus}
                        onChange={setSearchStatus}
                        placeholder={t('按 status 查询')}
                        style={{minWidth: 200}}
                        showClear
                    >
                        <Select.Option value='0'>{t('Unknown (0)')}</Select.Option>
                        <Select.Option value='1'>{t('Enabled (1)')}</Select.Option>
                        <Select.Option value='2'>{t('Manually Disabled (2)')}</Select.Option>
                        <Select.Option value='3'>{t('Auto Disabled (3)')}</Select.Option>
                    </Select>
                    <Select
                        value={searchType}
                        onChange={setSearchType}
                        placeholder={t('按 type 查询')}
                        style={{minWidth: 220}}
                        showClear
                        filter
                    >
                        {channelTypeOptions.map((opt) => (
                            <Select.Option key={opt.value} value={opt.value}>
                                {opt.label}
                            </Select.Option>
                        ))}
                    </Select>
                    <Space>
                        <Button type='primary' onClick={handleSearch} loading={loading || metaLoading}>
                            {t('查询')}
                        </Button>
                        <Button onClick={handleReset} disabled={loading || metaLoading}>
                            {t('重置')}
                        </Button>
                        <Button onClick={() => loadData(page, pageSize)} disabled={loading || metaLoading}>
                            {t('刷新')}
                        </Button>
                    </Space>
                </div>

                <div className='flex flex-wrap gap-2 items-center justify-end'>
                    <Button type='primary' theme='solid' onClick={openCreateDialog} disabled={metaLoading}>
                        {t('新增行')}
                    </Button>
                    <Button
                        type='warning'
                        onClick={openBatchEditDialog}
                        disabled={selectedRows.length === 0}
                    >
                        {t('批量修改')}
                    </Button>
                    <Button
                        type='secondary'
                        onClick={openBatchCopyDialog}
                        disabled={selectedRows.length !== 1}
                    >
                        {t('批量复制新增')}
                    </Button>
                    <Typography.Text type='tertiary'>
                        {t('共 {{total}} 行', {total})}
                    </Typography.Text>
                </div>
            </div>

            <Table
                columns={tableColumns}
                dataSource={rows}
                loading={loading || metaLoading}
                rowKey='id'
                scroll={{x: 'max-content'}}
                rowSelection={{
                    selectedRowKeys,
                    onChange: (keys, rs) => {
                        setSelectedRowKeys(keys);
                        setSelectedRows(rs);
                    },
                }}
                pagination={{
                    currentPage: page,
                    pageSize,
                    total,
                    pageSizeOpts: [10, 20, 50, 100, 200],
                    showSizeChanger: true,
                    onPageChange: (p) => loadData(p, pageSize),
                    onPageSizeChange: (s) => loadData(1, s),
                }}
            />

            <Modal
                title={dialogMode === 'create' ? t('新增行') : t('编辑行')}
                visible={editVisible}
                onCancel={() => setEditVisible(false)}
                onOk={submitEdit}
                size='large'
                centered
                maskClosable={false}
            >
                {renderFormGrid(
                    editForm,
                    setEditForm,
                    dialogMode === 'create'
                        ? t('将创建一行记录：只要填写的列会生效（空字符串会被当作未提供）')
                        : t('仅会提交有变化的列'),
                )}
            </Modal>

            <Modal
                title={t('批量修改')}
                visible={batchVisible}
                onCancel={() => setBatchVisible(false)}
                onOk={submitBatchEdit}
                size='large'
                centered
                maskClosable={false}
            >
                {renderFormGrid(
                    batchForm,
                    setBatchForm,
                    t('当前选中 {{count}} 行：只会更新你填写的列，留空的列不会变更。', {
                        count: selectedRows.length,
                    }),
                )}
            </Modal>

            <Modal
                title={t('批量复制新增')}
                visible={batchCopyVisible}
                onCancel={() => setBatchCopyVisible(false)}
                onOk={submitBatchCopy}
                centered
                maskClosable={false}
            >
                <div className='flex flex-col gap-2'>
                    <Typography.Text type='tertiary'>
                        {t('将以当前选中的这一行作为模板，按下面每行的 key 复制新增一行记录，仅 key 字段不同。')}
                    </Typography.Text>
                    <TextArea
                        value={batchCopyKeysText}
                        onChange={setBatchCopyKeysText}
                        autosize={{minRows: 6, maxRows: 12}}
                        placeholder={t('每行一个 key')}
                        className='font-mono'
                    />
                </div>
            </Modal>
        </div>
    );
};

export default ChannelMgr;
