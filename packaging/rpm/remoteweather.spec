Name:           remoteweather
Version:        %{_version}
Release:        1%{?dist}
Summary:        Weather station data collection and distribution system

License:        MIT
URL:            https://github.com/chrissnell/remoteweather
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  systemd-rpm-macros
Requires(pre):  shadow-utils
Requires:       systemd

%description
RemoteWeather is a weather station data collection and distribution system
that supports various weather station hardware and provides a web interface
for viewing weather data.

%prep
%setup -q

%install
mkdir -p %{buildroot}%{_bindir}
mkdir -p %{buildroot}%{_unitdir}
mkdir -p %{buildroot}/var/lib/remoteweather

install -m 755 remoteweather %{buildroot}%{_bindir}/remoteweather
install -m 644 packaging/systemd/remoteweather.service %{buildroot}%{_unitdir}/remoteweather.service

%pre
getent group remoteweather >/dev/null || groupadd -r remoteweather
getent passwd remoteweather >/dev/null || \
    useradd -r -g remoteweather -d /var/lib/remoteweather -s /sbin/nologin \
    -c "RemoteWeather daemon" remoteweather
exit 0

%post
%systemd_post remoteweather.service

%preun
%systemd_preun remoteweather.service

%postun
%systemd_postun_with_restart remoteweather.service

%files
%{_bindir}/remoteweather
%{_unitdir}/remoteweather.service
%attr(755,remoteweather,remoteweather) %dir /var/lib/remoteweather

%changelog
* Date Author <email> - version
- Initial package