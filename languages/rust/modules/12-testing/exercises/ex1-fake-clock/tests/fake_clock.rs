use chrono::{DateTime, Duration, TimeZone, Utc};
use m12_ex1_fake_clock::{Clock, FakeClock, SessionStore};

fn start() -> DateTime<Utc> {
    Utc.with_ymd_and_hms(2030, 1, 1, 9, 0, 0).unwrap()
}

#[test]
fn fake_clock_stands_still_until_advanced() {
    let clock = FakeClock::new(start());
    assert_eq!(clock.now(), start());
    assert_eq!(clock.now(), start());

    clock.advance(Duration::minutes(90));
    assert_eq!(
        clock.now(),
        Utc.with_ymd_and_hms(2030, 1, 1, 10, 30, 0).unwrap()
    );

    let later = Utc.with_ymd_and_hms(2031, 6, 1, 0, 0, 0).unwrap();
    clock.set(later);
    assert_eq!(clock.now(), later);
}

#[test]
fn clones_share_one_timeline() {
    let clock = FakeClock::new(start());
    let handle_for_component = clock.clone();
    clock.advance(Duration::seconds(5));
    assert_eq!(handle_for_component.now(), start() + Duration::seconds(5));
}

#[test]
fn sessions_are_valid_until_the_ttl_elapses() {
    let clock = FakeClock::new(start());
    let mut sessions = SessionStore::new(clock.clone(), Duration::minutes(30));
    let id = sessions.create("ada");
    assert_eq!(id, "s1");

    clock.advance(Duration::minutes(29) + Duration::seconds(59));
    assert_eq!(sessions.validate(&id).as_deref(), Some("ada"));
}

#[test]
fn a_session_dies_exactly_at_its_expiry() {
    let clock = FakeClock::new(start());
    let mut sessions = SessionStore::new(clock.clone(), Duration::minutes(30));
    let id = sessions.create("ada");

    clock.advance(Duration::minutes(30));
    assert_eq!(sessions.validate(&id), None);
    assert!(
        sessions.is_empty(),
        "expired sessions are removed when seen"
    );
}

#[test]
fn validation_slides_the_expiry() {
    let clock = FakeClock::new(start());
    let mut sessions = SessionStore::new(clock.clone(), Duration::minutes(30));
    let id = sessions.create("ada");

    for _ in 0..5 {
        clock.advance(Duration::minutes(20));
        assert_eq!(sessions.validate(&id).as_deref(), Some("ada"));
    }
    // 100 minutes after creation, still alive because it was used every 20 minutes
    clock.advance(Duration::minutes(31));
    assert_eq!(sessions.validate(&id), None);
}

#[test]
fn unknown_sessions_are_rejected() {
    let mut sessions = SessionStore::new(FakeClock::new(start()), Duration::minutes(30));
    assert_eq!(sessions.validate("nope"), None);
}

#[test]
fn purge_removes_only_dead_sessions() {
    let clock = FakeClock::new(start());
    let mut sessions = SessionStore::new(clock.clone(), Duration::minutes(30));
    sessions.create("old-1");
    sessions.create("old-2");
    clock.advance(Duration::minutes(20));
    let fresh = sessions.create("fresh");

    clock.advance(Duration::minutes(15)); // old ones are 35 minutes old, fresh is 15
    assert_eq!(sessions.purge_expired(), 2);
    assert_eq!(sessions.len(), 1);
    assert_eq!(sessions.validate(&fresh).as_deref(), Some("fresh"));
    assert_eq!(sessions.purge_expired(), 0);
}
