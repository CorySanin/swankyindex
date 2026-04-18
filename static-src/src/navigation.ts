interface FileEntry {
    filename: string;
    size: string;
    date: string;
    time: string;
    dl: number;
    dlTotal: number;
}

interface DirApiResponse {
    error?: string;
    path: string;
    subdirectories: string[];
    files: FileEntry[];
    titlePrefix: string;
    dl: boolean;
}

document.addEventListener("DOMContentLoaded", function () {

    function setLoaderVisibility(visible: boolean) {
        const loader = document.querySelector('.loader');
        loader?.classList[visible ? 'remove' : 'add'].apply(loader.classList, ['display-none']);
    }

    function getTBody() {
        return document.querySelector('table#list > tbody')
    }

    function getAllCheckboxes() {
        return document.querySelectorAll('table#list > tbody > tr > td.checkCol > input[type="checkbox"]');
    }

    function clearChildren(el?: Element) {
        const parent = el || getTBody();
        while (parent?.firstChild) {
            parent.removeChild(parent.lastChild!);
        }
    }

    function createDirRow(dirName: string, dl: boolean): HTMLAnchorElement {
        const naCol = '-';
        const tbody = getTBody();
        const tr = document.createElement('tr');
        tbody?.appendChild(tr);
        const tdLink = document.createElement('td');
        const tdSize = document.createElement('td');
        const tdDate = document.createElement('td');

        [tdLink, tdSize, tdDate].forEach(td => tr.appendChild(td));
        tdLink.classList.add('link', 'directory');
        tdSize.classList.add('na', 'size');
        tdDate.classList.add('na', 'date');
        const anchor = document.createElement('a');
        anchor.appendChild(document.createTextNode(anchor.href = `${dirName}/`));
        anchor.title = dirName;
        tdLink.appendChild(anchor);
        tdSize.appendChild(document.createTextNode(naCol));
        tdDate.appendChild(document.createTextNode(naCol));
        if (dl) {
            const tdDl = document.createElement('td');
            const tddlTotal = document.createElement('td');
            [tdDl, tddlTotal].forEach(td => tr.appendChild(td));
            tdDl.classList.add('na', 'dlcount', 'dlRecent');
            tddlTotal.classList.add('na', 'dlcount', 'dlTotal');
            tdDl.appendChild(document.createTextNode(naCol));
            tddlTotal.appendChild(document.createTextNode(naCol));
        }
        return anchor;
    }

    function createFileRow(fileEntry: FileEntry, dl: boolean) {
        const tbody = getTBody();
        const tr = document.createElement('tr');
        tbody?.appendChild(tr);
        const tdLink = document.createElement('td');
        const tdSize = document.createElement('td');
        const tdDate = document.createElement('td');
        const timeSpan = document.createElement('span');
        [tdLink, tdSize, tdDate].forEach(td => tr.appendChild(td));
        tdLink.classList.add('link');
        tdSize.classList.add('size');
        tdDate.classList.add('date');
        timeSpan.classList.add('time');
        const anchor = document.createElement('a');
        anchor.appendChild(document.createTextNode(anchor.href = `${fileEntry.filename}`));
        anchor.title = fileEntry.filename;
        tdLink.appendChild(anchor);
        tdSize.appendChild(document.createTextNode(fileEntry.size));
        tdDate.appendChild(document.createTextNode(`${fileEntry.date} `));
        tdDate.appendChild(timeSpan);
        timeSpan.appendChild(document.createTextNode(fileEntry.time));
        if (dl) {
            const tdDl = document.createElement('td');
            const tddlTotal = document.createElement('td');
            [tdDl, tddlTotal].forEach(td => tr.appendChild(td));
            tdDl.classList.add('dlcount', 'dlRecent');
            tddlTotal.classList.add('dlcount', 'dlTotal');
            tdDl.appendChild(document.createTextNode(`${fileEntry.dl}`));
            tddlTotal.appendChild(document.createTextNode(`${fileEntry.dlTotal}`));
        }
    }

    async function navigate(url: URL) {
        const reqPath = `${url.protocol}//${url.host}/.api/dir${url.pathname}`;
        try {
            const resp = await fetch(reqPath, {
                headers: {
                    'X-API-Version': '1'
                }
            });
            if (!resp.ok) {
                throw new Error(`Response status: ${resp.status}`);
            }
            const respBody: DirApiResponse = await resp.json();
            const anchors = respBody.subdirectories.map(dir => createDirRow(dir, respBody.dl));
            respBody.files.forEach(file => createFileRow(file, respBody.dl));
            document.querySelectorAll('span.pathname').forEach(pathContainer => {
                clearChildren(pathContainer);
                pathContainer.appendChild(document.createTextNode(respBody.path));
            });
            document.title = `${respBody.titlePrefix}${respBody.path}`;
            setUpLinks(anchors);
        }
        catch {
            window.location.assign(url.toString());
        }
        setLoaderVisibility(false);
        setButtonState();
    }

    function loadUrl(url: URL) {
        window.scrollTo(0, 0);
        setLoaderVisibility(true);
        setButtonState(false);
        clearChildren();
        return navigate(url);
    }

    function clickLink(e: PointerEvent) {
        e.preventDefault();
        const anchor: HTMLAnchorElement = e.target as unknown as HTMLAnchorElement;
        const url = new URL(anchor.href);
        window.history.pushState(anchor.href, "", url.pathname);
        return loadUrl(url);
    }

    function setUpLink(el: Element) {
        const anchor: HTMLAnchorElement = el as unknown as HTMLAnchorElement;
        anchor.addEventListener('click', clickLink);
    }

    function setUpLinks(elements: NodeListOf<Element> | Element[]) {
        elements.forEach(setUpLink);
        addCheckboxCols();
    }

    function addCheckboxCols() {
        const trs = document.querySelectorAll('table#list > tbody > tr');
        trs.forEach(tr => {
            const td = document.createElement('td');
            const link: HTMLAnchorElement | null = tr.querySelector('td.link > a');
            td.classList.add('checkCol');
            if (link?.href.endsWith('/')) {
                td.appendChild(document.createTextNode('-'));
            }
            else if (link) {
                const checkbox = document.createElement('input');
                checkbox.type = 'checkbox';
                checkbox.value = link.href.substring(link.href.lastIndexOf('/') + 1);
                td.appendChild(checkbox);
                td.addEventListener('click', e => {
                    e.target === checkbox || (checkbox.checked = !checkbox.checked);
                });
            }
            tr.prepend(td);
        });
    }

    function setButtonState(force?: boolean) {
        const btn = downloadButton!;
        if (typeof force === 'boolean') {
            btn.disabled = !force;
            return;
        }
        btn.disabled = getAllCheckboxes().length === 0;
    }

    window.addEventListener('popstate', event => {
        event.state && loadUrl(new URL(event.state));
    });

    window.history.pushState(window.location.href, "", (new URL(window.location.href)).pathname);
    setUpLinks(document.querySelectorAll(".link.directory > a"));
    const downloadButton = (function (element: Element | null) {
        const button = document.createElement('button');
        if (!element) {
            return button;
        }
        const th = document.createElement('th');
        th.classList.add('checkCol');
        button.appendChild(document.createTextNode('📥️'));
        button.title = 'Download selected';
        th.appendChild(button);
        element.prepend(th);
        return button;
    })(document.querySelector('table#list > thead > tr'));

    downloadButton.addEventListener('click', async _ => {
        const checkedFiles: string[] = [];
        const allFiles: string[] = [];
        setLoaderVisibility(true);
        getAllCheckboxes().forEach(c => {
            const chkbx = c as HTMLInputElement;
            chkbx.checked && checkedFiles.push(decodeURIComponent(chkbx.value));
            chkbx.checked = false;
            checkedFiles.length > 0 || allFiles.push(chkbx.value);
        });
        const files = checkedFiles.length > 0 ? checkedFiles : allFiles;
        setButtonState(false);
        const url = new URL(window.location.href);
        const resp = await fetch(`${url.protocol}//${url.host}/.api/zip`, {
            method: 'POST',
            headers: {
                'X-API-Version': '1'
            },
            body: JSON.stringify({
                directory: decodeURIComponent(url.pathname),
                files: files
            })
        });
        if (!resp.ok) {
            throw new Error(`Response status: ${resp.status}`);
        }
        const zip = URL.createObjectURL(await resp.blob());
        const anchor = document.createElement('a');
        anchor.href = zip;
        const contentDisposition = resp.headers.get('Content-Disposition') || 'f=files.zip';
        anchor.download = contentDisposition.substring(contentDisposition.indexOf('=') + 1);
        document.body.appendChild(anchor);
        anchor.click();
        setTimeout(function () {
            document.body.removeChild(anchor);
            window.URL.revokeObjectURL(zip);
            setButtonState();
            setLoaderVisibility(false);
        }, 0);
    });
});
