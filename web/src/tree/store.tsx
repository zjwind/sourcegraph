import * as immutable from 'immutable'
import 'rxjs/add/operator/scan'
import 'rxjs/add/operator/startWith'
import { BehaviorSubject } from 'rxjs/BehaviorSubject'
import { Subject } from 'rxjs/Subject'
import { Subscription } from 'rxjs/Subscription'
import { getParentDir } from 'sourcegraph/tree/util'

export interface TreeState {
    shownSubpaths: immutable.Set<string>
    selectedPath: string
    selectedDir: string
}

export interface TreeStore {
    getValue: () => TreeState
    setState: (s: TreeState) => void
    subscribe: (cb: ((s: TreeState) => void)) => Subscription
}

export function createTreeStore(initSelectedPath?: string): TreeStore {
    let shownSubpaths = immutable.Set<string>()
    let selectedPath = ''
    let selectedDir = ''
    if (initSelectedPath) {
        // Open all paths leading up to this one.
        let curr = ''
        const split = initSelectedPath.split('/')
        for (const part of split) {
            if (curr !== '') {
                curr += '/'
            }
            curr += part
            shownSubpaths = shownSubpaths.add(curr)
        }
        selectedPath = initSelectedPath
        selectedDir = getParentDir(initSelectedPath)
    }
    const initState: TreeState = { shownSubpaths, selectedPath, selectedDir }
    const actionSubject = new Subject<TreeState>()
    const actionDispatcher = func => (...args) => actionSubject.next(func(...args))

    const reducer = (state, action) => { // TODO(john): use immutable data structure
        switch (action.type) {
            case 'SET':
                return action.payload
            default:
                return state
        }
    }
    const setState: (t: TreeState) => void = actionDispatcher(payload => ({
        type: 'SET',
        payload
    }))

    const store: BehaviorSubject<TreeState> = new BehaviorSubject<TreeState>(initState)
    actionSubject.startWith(initState).scan(reducer).subscribe(store)

    return { getValue: () => store.getValue(), setState, subscribe: cb => store.subscribe(cb) }
}
