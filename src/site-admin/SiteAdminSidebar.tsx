import FeedIcon from '@sourcegraph/icons/lib/Feed'
import * as React from 'react'
import { Link } from 'react-router-dom'
import {
    SIDEBAR_BUTTON_CLASS,
    SidebarGroup,
    SidebarGroupHeader,
    SidebarGroupItems,
    SidebarNavItem,
} from '../components/Sidebar'
import { NavGroupDescriptor } from '../util/contributions'

export interface SiteAdminSideBarGroup extends NavGroupDescriptor {}

export type SiteAdminSideBarGroups = ReadonlyArray<SiteAdminSideBarGroup>

export interface SiteAdminSidebarProps {
    /** The items for the side bar, by group */
    groups: SiteAdminSideBarGroups
    className: string
}

/**
 * Sidebar for the site admin area.
 */
export const SiteAdminSidebar: React.SFC<SiteAdminSidebarProps> = ({ className, groups }) => (
    <div className={`site-admin-sidebar ${className}`}>
        {groups.map((group, i) => (
            <SidebarGroup key={i}>
                {group.header && <SidebarGroupHeader icon={group.header.icon} label={group.header.label} />}
                <SidebarGroupItems>
                    {group.items.map(
                        ({ label, to, exact, condition = () => true }) =>
                            condition({}) && (
                                <SidebarNavItem to={to} exact={exact} key={label}>
                                    {label}
                                </SidebarNavItem>
                            )
                    )}
                </SidebarGroupItems>
            </SidebarGroup>
        ))}

        <Link to="/api/console" className={SIDEBAR_BUTTON_CLASS}>
            <FeedIcon className="icon-inline" />
            API console
        </Link>
        <a href="/-/debug/" className={SIDEBAR_BUTTON_CLASS}>
            Instrumentation
        </a>
    </div>
)
