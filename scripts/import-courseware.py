#!/usr/bin/env python3
"""Offline administrator import of the reviewed 94-page course (Python stdlib only)."""
import argparse
import datetime
import json
from pathlib import Path
import shutil
import socket
import sqlite3


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--data-dir', required=True, type=Path)
    parser.add_argument('--server-address', required=True, help='Stopped server address, e.g. 127.0.0.1:8080')
    args = parser.parse_args()
    host, port = args.server_address.rsplit(':', 1)
    try:
        with socket.create_connection((host, int(port)), timeout=2):
            parser.error('请先停止应用服务，防止内存中的课堂状态覆盖导入结果。')
    except OSError:
        pass
    data = args.data_dir.resolve()
    dbfile = data / 'training.sqlite'
    if not dbfile.is_file():
        parser.error('数据目录尚未初始化，请先启动系统一次，再停止服务后导入。')
    source = Path(__file__).resolve().parents[1] / 'courseware' / 'official94'
    slides = json.loads((source / 'slides.json').read_text())
    if len(slides) != 94:
        parser.error('课件包页数错误')
    for i, slide in enumerate(slides, 1):
        expected = f'pdf-official94/page-{i:03}.png'
        if slide['id'] != f'P{i:03}' or slide['asset'] != expected:
            parser.error('课件包页序或资源路径错误')
        if not (source / f'page-{i:03}.png').is_file():
            parser.error('课件包缺少页面图片')
    timestamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
    backups = data / 'backups'
    backups.mkdir(mode=0o700, exist_ok=True)
    backup = backups / f'before-courseware-{timestamp}.sqlite'
    with sqlite3.connect(dbfile) as db:
        with sqlite3.connect(backup) as copy:
            db.backup(copy)
        backup.chmod(0o600)
        row = db.execute('SELECT payload FROM classroom WHERE id=1').fetchone()
        if not row:
            parser.error('课堂尚未初始化')
        state = json.loads(row[0])
        target = data / 'courseware' / f'pdf-official94-{timestamp}'
        target.mkdir(parents=True, mode=0o700)
        for i, slide in enumerate(slides, 1):
            filename = f'page-{i:03}.png'
            shutil.copyfile(source / filename, target / filename)
            (target / filename).chmod(0o600)
            slide['asset'] = f'{target.name}/{filename}'
        previous_revision = state['revision']
        state.update(slides=slides, projection='', closed=[], revision=previous_revision + 1)
        for learner in state['learners']:
            learner.update(pages=[], current='')
        state.setdefault('audit', []).append({
            'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
            'actor': 'admin-offline-import',
            'action': {'kind': 'courseware-replace', 'revision': previous_revision},
        })
        result = db.execute(
            'UPDATE classroom SET payload=? WHERE id=1 AND payload=?',
            (json.dumps(state, ensure_ascii=False).encode(), row[0]),
        )
        if result.rowcount != 1:
            raise RuntimeError('课堂状态发生变化，导入已取消')
        db.commit()
        print(f'Imported {len(slides)} pages; revision {state["revision"]}; backup: {backup}')


if __name__ == '__main__':
    main()
