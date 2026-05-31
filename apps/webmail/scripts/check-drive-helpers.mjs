import assert from 'node:assert/strict';

import {
  DRIVE_NODE_DRAG_MIME,
  DRIVE_NODE_DRAG_TEXT,
} from '../src/lib/drive/driveUtils.ts';
import {
  getDriveNodeDragPayload,
  getDriveUploadSourceLabel,
  loadDriveSortSetting,
  normalizeDroppedPath,
  parseDriveNodeIds,
  sortDriveNodes,
} from '../src/components/drive/driveViewHelpers.ts';

const nodes = [
  { id: 'file-b', node_type: 'file', name: 'B.txt', mime_type: 'text/plain', size: 20, status: 'active', created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-03T00:00:00Z' },
  { id: 'folder-a', node_type: 'folder', name: 'a folder', size: 0, status: 'active', created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-02T00:00:00Z' },
  { id: 'file-a', node_type: 'file', name: 'a.txt', mime_type: 'text/plain', size: 10, status: 'active', created_at: '2026-05-01T00:00:00Z', updated_at: '2026-05-04T00:00:00Z' },
];

assert.deepEqual(sortDriveNodes(nodes, 'typeName').map((node) => node.id), ['folder-a', 'file-a', 'file-b']);
assert.deepEqual(sortDriveNodes(nodes, 'updated').map((node) => node.id), ['file-a', 'file-b', 'folder-a']);
assert.deepEqual(sortDriveNodes(nodes, 'size').map((node) => node.id), ['file-b', 'file-a', 'folder-a']);
assert.deepEqual(sortDriveNodes(nodes, 'name').map((node) => node.id), ['folder-a', 'file-a', 'file-b']);

assert.equal(normalizeDroppedPath('\\folder//child/file.txt/'), 'folder/child/file.txt');

assert.deepEqual(parseDriveNodeIds(JSON.stringify({ nodeIds: ['a', 'b', 'a'] })), ['a', 'b']);
assert.deepEqual(parseDriveNodeIds('a,b,,c'), ['a', 'b', 'c']);
assert.deepEqual(parseDriveNodeIds('single'), ['single']);
assert.equal(parseDriveNodeIds(null), null);

const t = (key, values) => (values?.count ? `${key}:${values.count}` : key);
assert.equal(getDriveUploadSourceLabel('picker', t), 'sourceFilePicker');
assert.equal(getDriveUploadSourceLabel('folder', t), 'sourceFolderPicker');
assert.equal(getDriveUploadSourceLabel('drop', t), 'sourceDrop');

const previousLocalStorage = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: {
  value: '{}',
  getItem() { return this.value; },
  },
});
globalThis.localStorage.value = JSON.stringify({ driveSort: 'updated' });
assert.equal(loadDriveSortSetting(), 'updated');
globalThis.localStorage.value = JSON.stringify({ driveSort: 'unexpected' });
assert.equal(loadDriveSortSetting(), 'typeName');
globalThis.localStorage.value = '{bad-json';
assert.equal(loadDriveSortSetting(), 'typeName');
if (previousLocalStorage) {
  Object.defineProperty(globalThis, 'localStorage', previousLocalStorage);
} else {
  delete globalThis.localStorage;
}

const customDragPayload = JSON.stringify({ nodeIds: ['node-1', 'node-2'] });
const customDataTransfer = {
  getData(type) {
    return type === DRIVE_NODE_DRAG_MIME ? customDragPayload : '';
  },
};
assert.equal(getDriveNodeDragPayload(customDataTransfer), customDragPayload);

const textDataTransfer = {
  getData(type) {
    if (type === DRIVE_NODE_DRAG_MIME) return '';
    if (type === DRIVE_NODE_DRAG_TEXT) return 'nodes:node-3,node-4';
    return '';
  },
};
assert.equal(getDriveNodeDragPayload(textDataTransfer), 'node-3,node-4');

console.log('drive helper checks passed');
