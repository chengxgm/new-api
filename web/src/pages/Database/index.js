// 参考 Token 的写法，对 Database 页面进行了重构，将原有的编辑、新增、批量更新等功能合并到一个 SideSheet 中，
// 并使用多个 Card 做信息分块展示，保持与 Token 页面相似的布局和体验。

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Layout,
  Card,
  Space,
  Select,
  Button,
  Table,
  Empty,
  Spin,
  SideSheet,
  Typography,
  Tag,
  Popconfirm,
  Form,
  Input,
  Toast,
} from '@douyinfe/semi-ui';
import {
  IconSave,
  IconClose,
  IconPlusCircle,
  IconUserGroup,
  IconDelete,
  IconServer,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API } from '../../helpers';

const { Content } = Layout;
const { Title, Text } = Typography;

/**
 * 列元信息类型定义
 * @typedef {Object} ColumnMeta
 * @property {string} name           - 列名
 * @property {string} type           - 数据库字段类型
 * @property {boolean} [pk]          - 是否为主键
 * @property {boolean} [nullable]    - 是否可空
 * @property {*} [default]           - 默认值
 * @property {string} [extra]        - auto_increment 等
 */

/**
 * 根据 ColumnMeta 找到可用的 rowKey（优先主键 > id 字段 > 第一列）
 * @param {ColumnMeta[]} cols
 * @returns {string}
 */
const getRowKey = (cols) => {
  const pk = cols.find((c) => c.pk);
  if (pk) return pk.name;
  if (cols.some((c) => c.name === 'id')) return 'id';
  return cols[0]?.name || 'id'; // 兜底
};

const DatabaseManager = () => {
  const { t } = useTranslation();

  // 选择表
  const [tableList, setTableList] = useState([]);
  const [activeTable, setActiveTable] = useState('');

  // 列元信息
  const [columnsMeta, setColumnsMeta] = useState([]);
  const rowKey = useMemo(() => getRowKey(columnsMeta), [columnsMeta]);

  // 构造给 Table 用的列
  const [columns4Table, setColumns4Table] = useState([]);

  // 数据
  const [tableData, setTableData] = useState([]);

  // 加载和分页
  const [loading, setLoading] = useState(false);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });

  // 选中行
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);

  // Bulk update
  const [bulkUpdateVisible, setBulkUpdateVisible] = useState(false);
  const [bulkUpdateLoading, setBulkUpdateLoading] = useState(false);
  const [bulkUpdateField, setBulkUpdateField] = useState('');
  const [bulkUpdateValue, setBulkUpdateValue] = useState('');

  // SideSheet 显示控制
  const [sideSheetVisible, setSideSheetVisible] = useState(false);

  // 是否编辑模式 / 当前编辑记录
  const [isEdit, setIsEdit] = useState(false);
  const [editingRecord, setEditingRecord] = useState({});
  const [sideSheetLoading, setSideSheetLoading] = useState(false);

  // 外置 Form 引用
  const formApiRef = useRef(null);

  // 初始化 & 切换表
  useEffect(() => {
    getTables();
  }, []);

  useEffect(() => {
    if (activeTable) {
      // 先重置分页和已选行
      setPagination((prev) => ({ ...prev, current: 1 }));
      setSelectedRowKeys([]);
      getTableInfo(activeTable);
      getTableData(1, pagination.pageSize);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTable]);

  // 请求获取所有数据表
  const getTables = async () => {
    try {
      const { data } = await API.get('/api/database/tables');
      if (data.success) {
        setTableList(data.data || []);
      } else {
        Toast.error(data.message);
      }
    } catch (e) {
      Toast.error(e.message);
    }
  };

  // 请求指定表结构信息
  const getTableInfo = async (tableName) => {
    try {
      const { data } = await API.get(`/api/database/tables/${tableName}/info`);
      if (!data.success) {
        Toast.error(data.message);
        return;
      }
      // 统一处理返回结构
      const normalize = data.data.map((c) => {
        // SQLite 直接就是 { name, type, pk }
        if (c.name) {
          return {
            name: c.name,
            type: c.type || c.data_type,
            pk: !!c.pk || c.key === 'PRI',
            nullable: c.is_nullable === 'YES' || c.notnull === 0,
            default: c.column_default ?? c.dflt_value,
            extra: c.extra,
          };
        }
        // MySQL 的 DESCRIBE
        return {
          name: c.Field,
          type: c.Type,
          pk: c.Key === 'PRI',
          nullable: c.Null === 'YES',
          default: c.Default,
          extra: c.Extra,
        };
      });
      setColumnsMeta(normalize);

      // 构造表格列
      const dynamic = normalize.map((col) => {
        const columnConfig = {
          title: col.name,
          dataIndex: col.name,
          key: col.name,
          ellipsis: true,
          width: 150,
        };
        // 若字段类型包含 date 或 timestamp，则在前端进行日期格式化，避免不一致的转换问题
        if (
          col.type &&
          (col.type.toLowerCase().includes('date') ||
            col.type.toLowerCase().includes('timestamp'))
        ) {
          columnConfig.render = (text) => {
            if (!text) return '';
            const d = new Date(text);
            if (isNaN(d.getTime())) {
              // 如果无效，直接返回原始值
              return text;
            }
            // 使用UTC时间格式化，确保与后端一致
            return d.toISOString();
          };
        }

        return columnConfig;
      });

      // 末尾添加操作列
      dynamic.push({
        title: t('操作'),
        key: 'actions',
        width: 180,
        fixed: 'right',
        render: (_, record) => (
          <Space>
            <Button
              type="link"
              onClick={() => handleEditRecord(record)}
            >
              {t('编辑')}
            </Button>
            <Popconfirm
              title={t('确认删除?')}
              onConfirm={() => handleDelete(record[rowKey])}
            >
              <Button type="link" danger>
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      });

      setColumns4Table(dynamic);
    } catch (e) {
      Toast.error(e.message);
    }
  };

  // 请求拉取表数据（分页）
  const getTableData = async (page = 1, pageSize = 10) => {
    if (!activeTable) return;
    setLoading(true);
    try {
      const { data } = await API.get(`/api/database/tables/${activeTable}`, {
        params: { page, page_size: pageSize },
      });
if (data.success) {
  setTableData(Array.isArray(data.data) ? data.data : []);
  setPagination({ current: page, pageSize, total: data.total });
} else {
  Toast.error(data.message);
}
    } catch (e) {
      Toast.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  // 打开 SideSheet，创建模式
  const handleCreate = () => {
    setIsEdit(false);
    setEditingRecord({});
    setSideSheetVisible(true);
    // 延迟清表单
    setTimeout(() => {
      if (formApiRef.current) {
        formApiRef.current.reset();
      }
    });
  };

  // 打开 SideSheet，编辑模式
  const handleEditRecord = (record) => {
    setIsEdit(true);
    setEditingRecord(record);
    setSideSheetVisible(true);
    // 延迟填充表单
    setTimeout(() => {
      if (formApiRef.current) {
        formApiRef.current.reset();
        formApiRef.current.setValues(record);
      }
    });
  };

  // 删除记录
  const handleDelete = async (id) => {
    try {
      await API.delete(`/api/database/tables/${activeTable}/${id}`);
      Toast.success(t('删除成功'));
      // 如果删完后当前页没数据，则跳到上一页
      const left = pagination.total - 1;
      const newPage =
        (pagination.current - 1) * pagination.pageSize >= left
          ? Math.max(1, pagination.current - 1)
          : pagination.current;
      await getTableData(newPage, pagination.pageSize);
    } catch (e) {
      Toast.error(e.response?.data?.message || e.message);
    }
  };

  // 提交（新增 / 编辑）
  const handleSubmit = async () => {
    if (!activeTable) return;
    try {
      const values = await formApiRef.current.validate();
      setSideSheetLoading(true);

      if (isEdit) {
        // 编辑
        const recId = editingRecord?.[rowKey];
        await API.put(`/api/database/tables/${activeTable}/${recId}`, values);
        Toast.success(t('更新成功'));
      } else {
        // 新增
        await API.post(`/api/database/tables/${activeTable}`, values);
        Toast.success(t('创建成功'));
      }

      setSideSheetVisible(false);
      await getTableData(pagination.current, pagination.pageSize);
    } catch (err) {
      Toast.error(err.response?.data?.message || err.message);
    } finally {
      setSideSheetLoading(false);
    }
  };

  // 取消
  const handleCancel = () => {
    setSideSheetVisible(false);
  };

  // Bulk update
  const handleBulkUpdateOpen = () => {
    if (selectedRowKeys.length === 0) {
      Toast.warning('请先选择要修改的行');
      return;
    }
    setBulkUpdateVisible(true);
  };

  const handleBulkUpdateSubmit = async () => {
    if (!activeTable) return;
    if (!bulkUpdateField) {
      Toast.error('请选择要更新的字段');
      return;
    }
    setBulkUpdateLoading(true);
    try {
      await API.put(`/api/database/tables/${activeTable}/bulk-update`, {
        ids: selectedRowKeys,
        update: { [bulkUpdateField]: bulkUpdateValue },
      });
      Toast.success('批量更新成功');
      setBulkUpdateVisible(false);
      getTableData(pagination.current, pagination.pageSize);
    } catch (e) {
      Toast.error(e.response?.data?.message || e.message);
    } finally {
      setBulkUpdateLoading(false);
    }
  };

const handleBulkDelete = async () => {
  if (!activeTable) return;
  if (selectedRowKeys.length === 0) {
    Toast.warning('请先选择要删除的行');
    return;
  }
  try {
    await API.delete(`/api/database/tables/${activeTable}/bulk-delete`, {
      data: { pk: rowKey, ids: selectedRowKeys },
    });
    Toast.success('批量删除成功');
    getTableData(pagination.current, pagination.pageSize);
  } catch (e) {
    Toast.error(e.response?.data?.message || e.message);
  }
};

  return (
    <Content style={{ padding: 24 }}>
      <Spin spinning={loading}>
        <Card className="!rounded-2xl shadow-sm border-0 mb-6">
          <div
            className="flex items-center justify-between p-6 rounded-xl"
                  style={{
                      background: '#fff',
                    position: 'relative',
                  }}
          >
            <div className="text-left">
              <Title heading={4} className="m-0">
                {t('数据库管理')}
              </Title>
              <Text style={{ opacity: 0.8 }}>
                {t('可选择数据表并对其进行查看、编辑和新增等操作')}
              </Text>
            </div>
            <div>
              <Space>
                <Select
                  placeholder={t('选择数据表')}
                  style={{ width: 240 }}
                  value={activeTable}
                  onChange={(val) => setActiveTable(val)}
                  allowClear
                >
                  {tableList.map((tb) => (
                    <Select.Option key={tb} value={tb}>
                      {tb}
                    </Select.Option>
                  ))}
                </Select>
                <Button
                  theme="light"
                  type="primary"
                  icon={<IconServer />}
                  onClick={() => getTableData()}
                  disabled={!activeTable}
                  className="!rounded-full"
                >
                  {t('刷新')}
                </Button>
                <Button
                  theme="solid"
                  icon={<IconPlusCircle />}
                  onClick={handleCreate}
                  disabled={!activeTable}
                  className="!rounded-full"
                >
                  {t('新增记录')}
                </Button>
                <Button
                  theme="solid"
                  onClick={handleBulkUpdateOpen}
                  disabled={!activeTable}
                  className="!rounded-full"
                >
                  批量修改
                </Button>
<Popconfirm
  position="topRight"
  showArrow
  autoAdjustOverflow
  title="确认批量删除？"
  content="此操作不可逆，请确认是否删除"
  onConfirm={handleBulkDelete}
>
  <Button
    theme="solid"
    icon={<IconDelete />}
    disabled={!activeTable}
    className="!rounded-full"
  >
    批量删除
  </Button>
</Popconfirm>
              </Space>
            </div>
          </div>

          <div className="p-6">
            {activeTable ? (
              <Table
                columns={columns4Table}
                dataSource={tableData}
                rowKey={rowKey}
                scroll={{ x: 'max-content' }}
                pagination={{
                  current: pagination.current,
                  pageSize: pagination.pageSize,
                  total: pagination.total,
                  showSizeChanger: true,
                  pageSizeOptions: ['10', '20', '50', '100'],
                  onChange: (current, pageSize) => getTableData(current, pageSize),
                }}
                rowSelection={{
                  selectedRowKeys,
                  onChange: (keys) => setSelectedRowKeys(keys),
                }}
              />
            ) : (
              <Empty
                description={t('请选择一张数据表')}
                style={{ height: 300 }}
              />
            )}
          </div>
        </Card>

        {/* SideSheet for 新增/编辑 */}
        <SideSheet
          placement={isEdit ? 'right' : 'left'}
          title={
            <Space>
              {isEdit ? (
                <Tag color="blue" shape="circle">
                  {t('更新')}
                </Tag>
              ) : (
                <Tag color="green" shape="circle">
                  {t('新建')}
                </Tag>
              )}
              <Title heading={4} className="m-0">
                {isEdit ? t('编辑记录信息') : t('创建新的记录')}
              </Title>
            </Space>
          }
          visible={sideSheetVisible}
          width={600}
          footer={
            <div className="flex justify-end bg-white">
              <Space>
                <Button
                  theme="solid"
                  size="large"
                  className="!rounded-full"
                  onClick={handleSubmit}
                  icon={<IconSave />}
                  loading={sideSheetLoading}
                >
                  {t('提交')}
                </Button>
                <Button
                  theme="light"
                  size="large"
                  className="!rounded-full"
                  type="primary"
                  onClick={handleCancel}
                  icon={<IconClose />}
                >
                  {t('取消')}
                </Button>
              </Space>
            </div>
          }
          closeIcon={null}
          onCancel={handleCancel}
        >
          <Spin spinning={sideSheetLoading}>
            <div className="p-6">
              <Card className="!rounded-2xl shadow-sm border-0 mb-6">
                <div
                  className="flex items-center mb-4 p-6 rounded-xl"
                  style={{
                    background: '#fff',
                    position: 'relative',
                  }}
                >
                  <div className="absolute inset-0 overflow-hidden">
                    <div className="absolute -top-10 -right-10 w-40 h-40 bg-white opacity-5 rounded-full"></div>
                    <div className="absolute -bottom-8 -left-8 w-24 h-24 bg-white opacity-10 rounded-full"></div>
                  </div>
                  <div className="w-10 h-10 rounded-full bg-white/20 flex items-center justify-center mr-4 relative">
                    <IconUserGroup size="large" style={{ color: '#ffffff' }} />
                  </div>
                  <div className="relative">
                    <Text
                      style={{ color: '#ffffff' }}
                      className="text-lg font-medium"
                    >
                      {t('记录信息')}
                    </Text>
                    <div
                      style={{ color: '#ffffff' }}
                      className="text-sm opacity-80"
                    >
                      {t('填写或编辑该记录的基础字段信息')}
                    </div>
                  </div>
                </div>

                <Form
                  labelPosition="inset"
                  getFormApi={(api) => (formApiRef.current = api)}
                >
                  {columnsMeta.map((col) => {
                    const disabled =
                      col.pk || (col.extra && col.extra.includes('auto_increment'));
                    return (
                      <Form.Input
                        key={col.name}
                        field={col.name}
                        label={col.name}
                        disabled={disabled}
                        rules={[
                          {
                            required: !col.nullable && !disabled,
                            message: `${col.name} ${t('为必填字段')}`,
                          },
                        ]}
                      />
                    );
                  })}
                </Form>
              </Card>

              <Card className="!rounded-2xl shadow-sm border-0">
                <div
                  className="flex items-center mb-4 p-6 rounded-xl"
                  style={{
                    background: '#fff',
                    position: 'relative',
                  }}
                >
                  <div className="absolute inset-0 overflow-hidden">
                    <div className="absolute -top-10 -right-10 w-40 h-40 bg-white opacity-5 rounded-full"></div>
                    <div className="absolute -bottom-8 -left-8 w-24 h-24 bg-white opacity-10 rounded-full"></div>
                  </div>
                  <div className="w-10 h-10 rounded-full bg-white/20 flex items-center justify-center mr-4 relative">
                    <IconDelete size="large" style={{ color: '#ffffff' }} />
                  </div>
                  <div className="relative">
                    <Text
                      style={{ color: '#ffffff' }}
                      className="text-lg font-medium"
                    >
                      {t('注意事项')}
                    </Text>
                    <div
                      style={{ color: '#ffffff' }}
                      className="text-sm opacity-80"
                    >
                      {t('请谨慎操作记录的增删改，以免造成数据损失')}
                    </div>
                  </div>
                </div>
                <div className="p-4">
                  <Text type="tertiary" className="block text-xs">
                    {t('更多高级操作可在侧边栏批量进行')}
                  </Text>
                </div>
              </Card>
            </div>
          </Spin>
        </SideSheet>

        {/* SideSheet for 批量更新 */}
        <SideSheet
          visible={bulkUpdateVisible}
          onCancel={() => setBulkUpdateVisible(false)}
          title="批量更新字段"
          width={500}
          closeIcon={null}
          footer={
            <div style={{ textAlign: 'right' }}>
              <Space>
                <Button
                  loading={bulkUpdateLoading}
                  onClick={handleBulkUpdateSubmit}
                  type="primary"
                >
                  提交
                </Button>
                <Button onClick={() => setBulkUpdateVisible(false)}>取消</Button>
              </Space>
            </div>
          }
        >
          <div style={{ padding: 16 }}>
            <Form>
              <Form.Input
                field="bulkField"
                label="字段名"
                onChange={(val) => setBulkUpdateField(val)}
              />
              <Form.Input
                field="bulkValue"
                label="字段值"
                onChange={(val) => setBulkUpdateValue(val)}
              />
            </Form>
          </div>
        </SideSheet>
      </Spin>
    </Content>
  );
};

export default DatabaseManager;
